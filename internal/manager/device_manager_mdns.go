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
		Browse(dm.handleMDNSEvent,
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
	<-ctx.Done()
	if err := client.Close(); err != nil {
		log.Err(err).Msg("Failed to close mDNS browser")
	}
	log.Info("mDNS discovery stopped")
}

func (dm *DeviceManager) handleMDNSEvent(e zeroconf.Event) {
	if e.Service == nil || e.Op == zeroconf.OpUpdated {
		log.Printf("%s name=%s host=%s type=%s ip=%v port=%d ", e.Op.String(), e.Name, e.Hostname, e.Type.Name, e.Addrs, e.Port)
		return
	}

	serviceName := strings.Replace(e.Name, "\\@", "@", -1)
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
	log.Printf("%s name=%s type=%s ip=%s port=%d txt=%v", e.Op.String(), serviceName, serviceType, ip, e.Port, e.Text)

	switch serviceType {
	case mdnsServiceAppleMobdev2:
		macAddr := strings.Split(serviceName, "@")[0]
		lockdownDevices, err := loadLockdownDevices()
		if err != nil {
			log.Println(err)
			return
		}

		if lockdownDev, ok := lockdownDevices[macAddr]; ok {
			udid := lockdownDev.Name
			dm.devices.Store(udid, model.Device{
				ID:          utils.Md5(udid),
				Name:        host,
				ServiceName: serviceName,
				MacAddr:     macAddr,
				IP:          ip,
				UDID:        udid,
				Connection:  model.LockdownConnection,
				Status:      model.Paired,
			})
		}
	case mdnsServiceRemotePairing:
		// WARN:
		// when CheckDevicePaired close the rsd handshake, it may trigger mdns event again
		// so we need to check if the device has been checked before to avoid continuous sending mdns event
		if dm.HasCheckedDevice(ip, e.Port, host) {
			return
		}
		// serviceName will change every mdns event, so we can't use serviceName to ignore duplicate
		if v, err := dm.CheckDevicePaired(ip, e.Port); err == nil && v != nil {
			device := model.Device{
				ID:          v.Id,
				Name:        host,
				ServiceName: serviceName,
				MacAddr:     "",
				IP:          ip,
				Port:        e.Port,
				UDID:        v.UniqueDeviceID,
				Connection:  model.RemoteConnection,
				Status:      model.Paired,
				PairingFile: v.PairingFile,
			}
			device.ParseDeviceClass()
			dm.SaveDevice(device)

			// Trigger device connection callback
			dm.onDeviceConnected(device)
		} else if err != nil {
			log.Debugf("Failed to check device pairing: name=%s ip=%s err=%s", serviceName, ip, err.Error())
		}
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
			Connection:  model.RemoteConnection,
			Status:      model.Pairable,
		}
		device.ParseDeviceClass()
		dm.SaveDevice(device)
	}
}

func (dm *DeviceManager) handleMDNSGoodbye(serviceType string, serviceName string) {
	switch serviceType {
	case mdnsServiceAppleMobdev2:
		macAddr := strings.Split(serviceName, "@")[0]
		dm.DeleteDeviceByMacAddr(macAddr)
	case mdnsServiceRemotePairing:
		dm.DeleteDeviceByServiceName(serviceName, model.RemoteConnection)
	case mdnsServiceRemoteManualPairing:
		dm.DeleteDeviceByServiceName(serviceName, model.RemoteConnection)
	}
}

func (dm *DeviceManager) Scan() {
	dm.mu.Lock()
	if dm.cancel != nil {
		dm.cancel()
	}
	dm.mu.Unlock()

	// 等待上一个实例退出
	time.Sleep(time.Second)

	dm.devices.Range(func(k, v interface{}) bool {
		dm.devices.Delete(k)
		return true
	})

	go dm.Start()

	// 等待10秒获取最新mdns数据
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
					if e.Service == nil || e.Op != zeroconf.OpAdded {
						return
					}

					address, ok := firstAddrString(e.Addrs)
					if !ok {
						return
					}

					name := strings.Replace(e.Name, "\\@", "@", -1)
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

			serviceName := strings.Replace(e.Name, "\\@", "@", -1)
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
	// Prefer returning IPv4 addresses
	for _, addr := range addrs {
		if addr.IsValid() && !addr.IsUnspecified() && addr.Is4() {
			return addr.String(), true
		}
	}
	// If no IPv4 address is found, return the first valid address
	for _, addr := range addrs {
		if addr.IsValid() && !addr.IsUnspecified() {
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
	defer listenConn4.Close()

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
