//go:build !linux

package manager

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/betamos/zeroconf"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/utils"
	"github.com/miekg/dns"
)

const (
	discoverWaitTime = time.Second * 10
	mdnsIPv4Addr     = "224.0.0.251:5353"
	metaQueryName    = "_services._dns-sd._udp.local."

	mdnsServiceAppleMobdev2        = "_apple-mobdev2._tcp"
	mdnsServiceRemotePairing       = "_remotepairing._tcp"
	mdnsServiceRemoteManualPairing = "_remotepairing-manual-pairing._tcp"
)

type discoveredServiceType struct {
	serviceType string
	domain      string
}

func (dm *DeviceManager) Start() {
	dm.mu.Lock()
	dm.ctx, dm.cancel = context.WithCancel(context.Background())
	ctx := dm.ctx
	dm.mu.Unlock()

	client, err := zeroconf.New().
		Browse(func(e zeroconf.Event) { dm.handleMDNSEvent(ctx, e) },
			zeroconf.NewType(mdnsServiceAppleMobdev2),
			zeroconf.NewType(mdnsServiceRemotePairing),
			zeroconf.NewType(mdnsServiceRemoteManualPairing),
		).
		Open()
	if err != nil {
		log.Err(err).Msg("Failed to initialize mDNS browser")
		return
	}

	log.Info("mDNS discovery started...")
	defer func() {
		if err := client.Close(); err != nil {
			log.Err(err).Msg("Failed to close mDNS browser")
		}
		log.Info("mDNS discovery stopped")
	}()

	// Reopen interfaces after Wi-Fi changes or waking a Mac from sleep.
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			client.Reload()
		}
	}
}

func (dm *DeviceManager) handleMDNSEvent(ctx context.Context, e zeroconf.Event) {
	if e.Service == nil || ctx.Err() != nil {
		return
	}

	serviceName := strings.ReplaceAll(e.Name, "\\@", "@")
	serviceType := e.Type.Name

	// goodbye event doesn't contain host and ip address
	if e.Op == zeroconf.OpRemoved {
		log.Printf("%s name=%s host=%s type=%s ip=%v port=%d ", e.Op.String(), serviceName, e.Hostname, serviceType, e.Addrs, e.Port)
		dm.handleMDNSGoodbye(serviceType, serviceName)
		return
	}

	ip, ok := firstAddrString(e.Addrs)
	if !ok {
		log.Printf("mDNS service has no address: name=%s type=%s", serviceName, serviceType)
		return
	}

	host := dm.parseName(e.Hostname)
	log.Printf("%s name=%s type=%s ip=%s port=%d", e.Op.String(), serviceName, serviceType, ip, e.Port)

	switch serviceType {
	case mdnsServiceAppleMobdev2:
		macAddr := strings.Split(serviceName, "@")[0]
		dm.lookupLockdownDevice(ctx, serviceName, macAddr, host, ip)
	case mdnsServiceRemotePairing:
		// WARN:
		// when open remote pairing port and close the rsd handshake, it may trigger mdns event again
		// so we need to check if the device has been checked before to avoid continuous sending mdns event
		identifier := dm.parseTextRecordIndentifier(e.Text)
		if identifier == "" {
			log.Warnf("Remote pairing service missing identifier: name=%s ip=%s", serviceName, ip)
			return
		}

		authTag := dm.parseTextRecordAuthTag(e.Text)
		if authTag == "" {
			log.Warnf("Remote pairing service missing auth tag: name=%s ip=%s", serviceName, ip)
			return
		}
		dm.rememberRemotePairingService(serviceName, identifier)

		// Throttle duplicate events for the same device: the iPhone
		// re-announces the remote pairing service every few seconds, and
		// every Add event would otherwise run a slow find-pairing subprocess.
		// Connection metadata is still refreshed so a device that
		// reconnects without a goodbye reflects its new address immediately.
		if dm.checkPairingThrottle(identifier) {
			dm.updateRemotePairingDevice(serviceName, ip, e.Port)
			return
		}

		// Run the slow pairing check off the event loop: the loop stays
		// responsive under event bursts, and a Remove event can cancel the
		// check (killing the plumesign subprocess) to release the goroutine.
		go dm.checkRemotePairingAsync(ctx, identifier, authTag, serviceName, host, ip, e.Port)
	case mdnsServiceRemoteManualPairing:
		name := serviceName
		if txtName := dm.parseTextRecordName(e.Text); txtName != "" {
			name = txtName
		}
		identifier := dm.parseTextRecordIndentifier(e.Text)
		if identifier == "" {
			log.Warnf("Remote manual pairing service missing identifier: name=%s ip=%s", serviceName, ip)
			return
		}
		// use serviceName to ignore duplicate
		id := utils.Md5(serviceName)
		device := model.Device{
			ID:          id,
			Name:        name,
			ServiceName: serviceName,
			MacAddr:     "",
			IP:          ip,
			Port:        e.Port,
			UDID:        identifier,
			Connection:  model.DeviceConnectionRemote,
			Status:      model.Pairable,
			DiscoveryAt: time.Now(),
		}
		device.ParseDeviceClass()
		dm.SaveDevice(device)
	}
}

func (dm *DeviceManager) handleMDNSGoodbye(serviceType string, serviceName string) {
	switch serviceType {
	case mdnsServiceAppleMobdev2:
		macAddr := strings.Split(serviceName, "@")[0]
		dm.cancelLockdownLookup(serviceName)
		dm.DeleteDeviceByMacAddr(macAddr)
	case mdnsServiceRemotePairing:
		// Cancel any in-flight pairing check and clear the throttle for
		// this device: a disconnect and reconnect within the throttle
		// window must not be skipped, otherwise the device never
		// reappears on the home page.
		dm.removeRemotePairingService(serviceName)
		dm.DeleteDeviceByServiceName(serviceName, model.DeviceConnectionRemote)
	case mdnsServiceRemoteManualPairing:
		dm.DeleteDeviceByServiceName(serviceName, model.DeviceConnectionRemote)
	}
}

// lockdownLookupDelays are the waits before each pair record lookup. macOS
// usbmuxd discovers the same mobdev2 announcement on its own and may not have
// attached the device, and so cannot serve its pair record, when atvloadly
// receives the announcement.
var lockdownLookupDelays = []time.Duration{0, time.Second, 3 * time.Second, 6 * time.Second}

// lookupLockdownDevice adds the device announced by a mobdev2 service once
// its pair record is available. A newer announcement, a goodbye or a rescan
// cancels the lookup. The lookup is registered before this returns, so a
// goodbye handled right after the announcement always cancels it.
func (dm *DeviceManager) lookupLockdownDevice(ctx context.Context, serviceName, macAddr, host, ip string) {
	lookupCtx, cancel := context.WithCancel(ctx)
	lookup := &pairingCheck{cancel: cancel}
	dm.pairingMu.Lock()
	if prev, ok := dm.lockdownLookups[serviceName]; ok {
		prev.cancel()
	}
	dm.lockdownLookups[serviceName] = lookup
	dm.pairingMu.Unlock()
	go dm.runLockdownLookup(lookupCtx, lookup, serviceName, macAddr, host, ip)
}

func (dm *DeviceManager) runLockdownLookup(lookupCtx context.Context, lookup *pairingCheck, serviceName, macAddr, host, ip string) {
	defer func() {
		lookup.cancel()
		dm.pairingMu.Lock()
		if dm.lockdownLookups[serviceName] == lookup {
			delete(dm.lockdownLookups, serviceName)
		}
		dm.pairingMu.Unlock()
	}()

	for _, delay := range lockdownLookupDelays {
		select {
		case <-lookupCtx.Done():
			return
		case <-time.After(delay):
		}
		// select picks randomly when both cases are ready.
		if lookupCtx.Err() != nil {
			return
		}
		lockdownDevices, err := loadLockdownDevices()
		if err != nil {
			log.Println(err)
			continue
		}
		lockdownDev, ok := lockdownDevices[macAddr]
		if !ok {
			continue
		}

		udid := lockdownDev.Name
		// Remote pairing wins over lockdown for the same device. Linux deletes
		// the lockdown record once remote pairing succeeds, but macOS owns its
		// records, so later mobdev2 announcements still match them.
		if existing, ok := dm.GetDeviceByUDID(udid); ok && existing.Connection == model.DeviceConnectionRemote {
			return
		}
		log.Debugf("add lockdown device >> %v", lockdownDev)
		device := model.Device{
			ID:          utils.Md5(udid),
			Name:        host,
			ServiceName: serviceName,
			MacAddr:     macAddr,
			IP:          ip,
			UDID:        udid,
			Connection:  model.DeviceConnectionLockdown,
			Status:      model.Paired,
			DiscoveryAt: time.Now(),
		}
		device.ParseDeviceClass()
		// Save under pairingMu: a goodbye cancels the lookup under the same
		// lock before deleting the device, so it cannot be re-added after.
		dm.pairingMu.Lock()
		saved := lookupCtx.Err() == nil
		if saved {
			dm.SaveDevice(device)
		}
		dm.pairingMu.Unlock()
		if saved {
			dm.onDeviceConnected(device)
		}
		return
	}
	log.Debugf("no pair record for mobdev2 service %s", serviceName)
}

func (dm *DeviceManager) cancelLockdownLookup(serviceName string) {
	dm.pairingMu.Lock()
	defer dm.pairingMu.Unlock()
	if lookup, ok := dm.lockdownLookups[serviceName]; ok {
		lookup.cancel()
		delete(dm.lockdownLookups, serviceName)
	}
}

func (dm *DeviceManager) Scan() {
	dm.Stop()

	time.Sleep(time.Second)

	dm.devices.Range(func(k, v interface{}) bool {
		dm.devices.Delete(k)
		return true
	})

	go dm.Start()

	timer := time.NewTimer(discoverWaitTime)
	<-timer.C
}

func (dm *DeviceManager) ScanServices(ctx context.Context, callback func(serviceType string, name string, host string, address string, port uint16, txt [][]byte)) error {
	if callback == nil {
		return nil
	}

	typeCh := make(chan discoveredServiceType, 32)
	errCh := make(chan error, 1)

	go func() {
		errCh <- discoverAllServiceTypes(ctx, typeCh)
		close(typeCh)
	}()

	clients := make(map[string]*zeroconf.Client)

	closeClient := func(key string) {
		if client, ok := clients[key]; ok {
			if err := client.Close(); err != nil {
				log.Err(err).Msgf("Failed to close mDNS browser for %s", key)
			}
			delete(clients, key)
		}
	}

	defer func() {
		for key := range clients {
			closeClient(key)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errCh:
			if err != nil && ctx.Err() == nil {
				return err
			}
			return nil
		case discovered, ok := <-typeCh:
			if !ok {
				return nil
			}

			browserKey := discovered.serviceType + "." + discovered.domain
			if _, exists := clients[browserKey]; exists {
				continue
			}

			browserType := discovered.serviceType
			if discovered.domain != "" {
				browserType = browserType + "." + discovered.domain
			}

			client, err := zeroconf.New().
				Browse(func(e zeroconf.Event) {
					if e.Service == nil || e.Op == zeroconf.OpRemoved {
						return
					}

					address, ok := firstAddrString(e.Addrs)
					if !ok {
						return
					}

					name := strings.ReplaceAll(e.Name, "\\@", "@")
					callback(e.Type.Name, name, e.Hostname, address, e.Port, toTxtBytes(e.Text))
				}, zeroconf.NewType(browserType)).
				Open()
			if err != nil {
				log.Err(err).Msgf("Failed to browse mDNS service type: %s", browserType)
				continue
			}

			clients[browserKey] = client
		}
	}
}

func (dm *DeviceManager) ScanWirelessDevices(ctx context.Context, timeout time.Duration) ([]model.Device, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if timeout > 10*time.Second {
		timeout = 10 * time.Second
	}
	scanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var (
		mu      sync.Mutex
		deviceM = map[string]model.Device{}
	)

	client, err := zeroconf.New().
		Browse(func(e zeroconf.Event) {
			if e.Service == nil {
				return
			}

			serviceName := strings.ReplaceAll(e.Name, "\\@", "@")
			if e.Op == zeroconf.OpRemoved {
				mu.Lock()
				delete(deviceM, serviceName)
				mu.Unlock()
				return
			}

			ip, ok := firstAddrString(e.Addrs)
			if !ok {
				return
			}

			host := dm.parseName(e.Hostname)
			device := model.Device{
				ID:          utils.Md5(serviceName),
				Name:        host,
				ServiceName: serviceName,
				MacAddr:     "",
				IP:          ip,
				Port:        e.Port,
				UDID:        "",
				Status:      model.Pairable,
			}
			device.ParseDeviceClass()

			mu.Lock()
			deviceM[serviceName] = device
			mu.Unlock()
		}, zeroconf.NewType(mdnsServiceRemotePairing)).
		Open()
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			log.Err(closeErr).Msg("Failed to close mDNS browser")
		}
	}()

	<-scanCtx.Done()

	mu.Lock()
	defer mu.Unlock()

	devices := make([]model.Device, 0, len(deviceM))
	for _, device := range deviceM {
		devices = append(devices, device)
	}

	return devices, nil
}

func firstAddrString(addrs []netip.Addr) (string, bool) {
	// Only a private LAN IPv4 is usable: plumesign cannot connect to the
	// global IPv6 or public addresses a device may announce.
	for _, addr := range addrs {
		if isLanAddr(addr) {
			return addr.String(), true
		}
	}
	return "", false
}

func toTxtBytes(txt []string) [][]byte {
	if len(txt) == 0 {
		return nil
	}

	out := make([][]byte, 0, len(txt))
	for _, item := range txt {
		out = append(out, []byte(item))
	}

	return out
}

func discoverAllServiceTypes(ctx context.Context, out chan<- discoveredServiceType) error {
	listenAddr4, err := net.ResolveUDPAddr("udp4", mdnsIPv4Addr)
	if err != nil {
		return err
	}

	listenConn4, err := net.ListenMulticastUDP("udp4", nil, listenAddr4)
	if err != nil {
		return err
	}
	defer closeUDPConn(listenConn4) //nolint:errcheck

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	known := map[string]struct{}{}
	msgCh := make(chan *dns.Msg, 32)
	errCh := make(chan error, 2)

	ipv4Targets := []*net.UDPAddr{listenAddr4}

	if err := sendMetaQuery(listenConn4, ipv4Targets); err != nil {
		log.Err(err).Msg("Failed to send mDNS IPv4 meta query")
	}

	go readMDNSMessages(ctx, listenConn4, msgCh, errCh)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := sendMetaQuery(listenConn4, ipv4Targets); err != nil {
				log.Err(err).Msg("Failed to send mDNS IPv4 meta query")
			}
		case err := <-errCh:
			if err != nil && ctx.Err() == nil {
				return err
			}
		case msg := <-msgCh:
			records := append(msg.Answer, msg.Ns...)
			records = append(records, msg.Extra...)

			for _, rr := range records {
				ptr, ok := rr.(*dns.PTR)
				if !ok || !strings.EqualFold(ptr.Hdr.Name, metaQueryName) || ptr.Hdr.Ttl == 0 {
					continue
				}

				serviceType, domain := parseDiscoveredServiceType(ptr.Ptr)
				if serviceType == "" {
					continue
				}

				key := serviceType + "." + domain
				if _, exists := known[key]; exists {
					continue
				}
				known[key] = struct{}{}

				select {
				case out <- discoveredServiceType{serviceType: serviceType, domain: domain}:
				case <-ctx.Done():
					return nil
				}
			}
		}
	}
}

func readMDNSMessages(ctx context.Context, conn *net.UDPConn, out chan<- *dns.Msg, errCh chan<- error) {
	buf := make([]byte, 65535)

	for {
		if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			select {
			case errCh <- err:
			case <-ctx.Done():
			}
			return
		}

		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-ctx.Done():
					return
				default:
				}
				continue
			}
			select {
			case errCh <- err:
			case <-ctx.Done():
			}
			return
		}

		msg := new(dns.Msg)
		if err := msg.Unpack(buf[:n]); err != nil {
			continue
		}

		select {
		case out <- msg:
		case <-ctx.Done():
			return
		}
	}
}

func sendMetaQuery(conn *net.UDPConn, targets []*net.UDPAddr) error {
	msg := new(dns.Msg)
	msg.SetQuestion(metaQueryName, dns.TypePTR)
	msg.RecursionDesired = false

	data, err := msg.Pack()
	if err != nil {
		return err
	}

	for _, target := range targets {
		if _, err := conn.WriteToUDP(data, target); err != nil {
			return err
		}
	}

	return nil
}

func closeUDPConn(conn *net.UDPConn) {
	_ = conn.Close()
}

func parseDiscoveredServiceType(ptr string) (string, string) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(ptr), ".")
	parts := strings.Split(trimmed, ".")
	if len(parts) < 3 {
		return "", ""
	}

	serviceType := parts[0] + "." + parts[1]
	domain := strings.Join(parts[2:], ".")
	if !strings.HasPrefix(parts[0], "_") || !strings.HasPrefix(parts[1], "_") {
		return "", ""
	}

	return serviceType, domain
}

func (dm *DeviceManager) parseTextRecord(txt []string) map[string]string {
	result := make(map[string]string)
	for _, item := range txt {
		kv := strings.SplitN(item, "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}

func (dm *DeviceManager) parseTextRecordName(txt []string) string {
	result := dm.parseTextRecord(txt)
	if name, ok := result["name"]; ok {
		return name
	}
	return ""
}

func (dm *DeviceManager) parseTextRecordIndentifier(txt []string) string {
	result := dm.parseTextRecord(txt)
	if id, ok := result["identifier"]; ok {
		return id
	}
	return ""
}

func (dm *DeviceManager) parseTextRecordAuthTag(txt []string) string {
	result := dm.parseTextRecord(txt)
	if authTag, ok := result["authTag"]; ok {
		return authTag
	}
	return ""
}
