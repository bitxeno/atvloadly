//go:build linux

package manager

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/utils"
	"github.com/godbus/dbus/v5"
	"github.com/holoplot/go-avahi"
)

const (
	mdnsServiceAppleMobdev2        = "_apple-mobdev2._tcp"
	mdnsServiceRemotePairing       = "_remotepairing._tcp"
	mdnsServiceRemoteManualPairing = "_remotepairing-manual-pairing._tcp"
	mdnsServiceDomain              = "local"

	// avahiRestartDelay is how long discovery waits before rebuilding its
	// browsers after the Avahi connection dropped.
	avahiRestartDelay = 5 * time.Second
)

// errAvahiBrowserFreed reports that a discovery browser was freed because the
// provider dropped a dead D-Bus connection.
var errAvahiBrowserFreed = errors.New("avahi browser freed")

// avahiDaemon is the process-wide Avahi connection.
//
// go-avahi ties a Server to a D-Bus connection: Server.Close() closes the
// connection, and ServerNew() starts a signal dispatch goroutine that only
// Server.Close() stops. Creating a Server per operation therefore leaks a
// connection, a goroutine and a registered signal channel every time. Sharing
// dbus.SystemBus() instead is not an option: godbus forbids Close on shared
// connections, and go-avahi never removes its signal channel, so the server
// could never be cleaned up.
//
// One long-lived Server avoids both problems. Browsers are still created per
// operation and freed when it ends.
var avahiDaemon = newAvahiServerProvider()

type avahiServerProvider struct {
	mu     sync.Mutex
	server *avahi.Server
	conn   *dbus.Conn
}

func newAvahiServerProvider() *avahiServerProvider {
	return &avahiServerProvider{}
}

// get returns the shared Avahi server, reconnecting when the D-Bus connection
// has died (e.g. avahi-daemon restarted). The returned server is owned by the
// provider and must not be closed by the caller.
func (p *avahiServerProvider) get() (*avahi.Server, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.server != nil && p.conn.Connected() {
		return p.server, nil
	}

	if p.server != nil {
		// The connection is dead. Closing frees the browsers registered on
		// it, which makes their channels readable-as-closed so the
		// operations using them return instead of hanging. Safe to close:
		// this connection is private to the provider.
		p.server.Close()
		p.server, p.conn = nil, nil
	}

	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("cannot get system bus: %v", err)
	}

	server, err := avahi.ServerNew(conn)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("avahi new failed: %v", err)
	}

	p.server, p.conn = server, conn
	return server, nil
}

// 需要依赖socket套接字：
// /var/run/dbus
// /var/run/avahi-daemon
func (dm *DeviceManager) Start() {
	dm.mu.Lock()
	dm.ctx, dm.cancel = context.WithCancel(context.Background())
	ctx := dm.ctx
	dm.mu.Unlock()

	// A browser freed because the provider dropped a dead connection ends
	// runDiscovery. Reconnect and restart discovery instead of leaving the
	// daemon permanently without device discovery.
	for {
		server, err := avahiDaemon.get()
		if err != nil {
			log.Err(err).Msg("Avahi server unavailable")
		} else {
			stopped := dm.runDiscovery(ctx, server)
			if ctx.Err() != nil {
				log.Info("Avahi discovery stopped")
				return
			}
			log.Warnf("Avahi discovery interrupted: %v, restarting in %s", stopped, avahiRestartDelay)
		}

		// Back off so a permanently unavailable avahi-daemon cannot spin
		// the process.
		select {
		case <-ctx.Done():
			log.Info("Avahi discovery stopped")
			return
		case <-time.After(avahiRestartDelay):
		}
	}
}

// runDiscovery consumes the three device browsers until the context is
// cancelled or a browser is freed. It returns why it stopped.
func (dm *DeviceManager) runDiscovery(ctx context.Context, server *avahi.Server) error {
	// Free the browsers when discovery stops. The shared server itself
	// stays open for the next Start() and for concurrent scans.
	browsers := newAvahiBrowsers(server)
	defer browsers.Close()

	host, err := server.GetHostName()
	if err != nil {
		log.Err(err).Msgf("GetHostName() failed: ")
	}
	log.Debugf("GetHostName(): %s", host)

	fqdn, err := server.GetHostNameFqdn()
	if err != nil {
		log.Err(err).Msgf("GetHostNameFqdn() failed: ")
	}
	log.Debugf("GetHostNameFqdn(): %s", fqdn)

	s, err := server.GetAlternativeHostName(host)
	if err != nil {
		log.Err(err).Msgf("GetAlternativeHostName() failed: ")
	}
	log.Debugf("GetAlternativeHostName(): %s", s)

	i, err := server.GetAPIVersion()
	if err != nil {
		log.Err(err).Msgf("GetAPIVersion() failed: ")
	}
	log.Debugf("GetAPIVersion(): %v", i)

	hn, err := server.ResolveHostName(avahi.InterfaceUnspec, avahi.ProtoUnspec, fqdn, avahi.ProtoUnspec, 0)
	if err != nil {
		log.Err(err).Msgf("ResolveHostName() failed: ")
	}
	log.Debugf("ResolveHostName: %v", hn)

	sbAppleMobdev := browsers.newServiceBrowser(avahi.InterfaceUnspec, avahi.ProtoUnspec, mdnsServiceAppleMobdev2, mdnsServiceDomain)
	if sbAppleMobdev == nil {
		return fmt.Errorf("apple-mobdev2 browser unavailable")
	}

	sbRemotePairing := browsers.newServiceBrowser(avahi.InterfaceUnspec, avahi.ProtoUnspec, mdnsServiceRemotePairing, mdnsServiceDomain)
	if sbRemotePairing == nil {
		return fmt.Errorf("remote pairing browser unavailable")
	}

	sbRemoteManualPairing := browsers.newServiceBrowser(avahi.InterfaceUnspec, avahi.ProtoUnspec, mdnsServiceRemoteManualPairing, mdnsServiceDomain)
	if sbRemoteManualPairing == nil {
		return fmt.Errorf("remote manual pairing browser unavailable")
	}

	log.Info("Avahi discovery started...")

	// A closed channel means the browser was freed, which happens when the
	// provider drops a dead connection. Return instead of spinning: on a
	// closed channel every receive succeeds immediately with a zero value.
	for {
		select {
		case <-ctx.Done():
			return nil
		case service, ok := <-sbAppleMobdev.AddChannel:
			if !ok {
				return errAvahiBrowserFreed
			}
			service, err := server.ResolveService(service.Interface, service.Protocol, service.Name,
				service.Type, service.Domain, avahi.ProtoUnspec, 0)
			if err != nil {
				log.Err(err).Msgf("Failed to resolve service: name=%s type=%s", service.Name, service.Type)
				continue
			}
			log.Printf("%s name=%s type=%s ip=%s port=%d txt=%v", "[+]", service.Name, service.Type, service.Address, service.Port, dm.parseTextRecord(service.Txt))

			macAddr := strings.Split(service.Name, "@")[0]
			name := dm.parseName(service.Host)
			// 检查是否已连接
			lockdownDevices, err := loadLockdownDevices()
			if err != nil {
				log.Err(err).Msg("loadLockdownDevices error: ")
				continue
			}
			log.Tracef("lockdown devices count >> %v", len(lockdownDevices))

			// 添加已连接设备，TODO：handshake检测是否可真实连接
			if lockdownDev, ok := lockdownDevices[macAddr]; ok {
				log.Debugf("add lockdown device >> %v", lockdownDev)
				udid := lockdownDev.Name
				device := model.Device{
					ID:          utils.Md5(udid),
					Name:        name,
					ServiceName: service.Name,
					MacAddr:     macAddr,
					IP:          service.Address,
					UDID:        udid,
					Connection:  model.DeviceConnectionLockdown,
					Status:      model.Paired,
					DiscoveryAt: time.Now(),
				}
				device.ParseDeviceClass()

				dm.SaveDevice(device)

				// Trigger device connection callback
				dm.onDeviceConnected(device)
			}
		case service, ok := <-sbAppleMobdev.RemoveChannel:
			if !ok {
				return errAvahiBrowserFreed
			}
			log.Printf("%s name=%s type=%s ip=%s port=%d txt=%v", "[-]", service.Name, service.Type, service.Address, service.Port, dm.parseTextRecord(service.Txt))

			macAddr := strings.Split(service.Name, "@")[0]
			dm.DeleteDeviceByMacAddr(macAddr)
		case service, ok := <-sbRemotePairing.AddChannel:
			if !ok {
				return errAvahiBrowserFreed
			}
			service, err := server.ResolveService(service.Interface, service.Protocol, service.Name,
				service.Type, service.Domain, avahi.ProtoUnspec, 0)
			if err != nil {
				log.Err(err).Msgf("Failed to resolve service: name=%s type=%s", service.Name, service.Type)
				continue
			}
			log.Printf("%s name=%s type=%s ip=%s port=%d txt=%v", "[+]", service.Name, service.Type, service.Address, service.Port, dm.parseTextRecord(service.Txt))

			name := dm.parseName(service.Host)

			identifier := dm.parseTextRecordIndentifier(service.Txt)
			if identifier == "" {
				log.Warnf("Remote pairing service missing identifier: name=%s ip=%s", service.Name, service.Address)
				continue
			}

			authTag := dm.parseTextRecordAuthTag(service.Txt)
			if authTag == "" {
				log.Warnf("Remote pairing service missing auth tag: name=%s ip=%s", service.Name, service.Address)
				continue
			}

			// ItemRemove does not include TXT records, so retain the identifier
			// while this service is present. The remove path must use the same
			// identifier as the pairing check and throttle maps.
			dm.rememberRemotePairingService(service.Name, identifier)

			// The iPhone re-announces/withdraws the remote pairing service
			// every few seconds. The throttle skips duplicate events so the
			// slow find-pairing subprocess cannot stall the avahi event
			// loop (which would back up the dbus signal channel and leak
			// one goroutine per signal). Connection metadata is still
			// refreshed so a device that reconnects without a goodbye
			// reflects its new address immediately.
			if dm.checkPairingThrottle(identifier) {
				dm.updateRemotePairingDevice(service.Name, service.Address, service.Port)
				continue
			}

			// Run the slow pairing check off the event loop: the loop
			// stays responsive under event bursts, and a Remove event can
			// cancel the check (killing the plumesign subprocess) to
			// release the goroutine immediately.
			go dm.checkRemotePairingAsync(ctx, identifier, authTag, service.Name, name, service.Address, service.Port)
		case service, ok := <-sbRemotePairing.RemoveChannel:
			if !ok {
				return errAvahiBrowserFreed
			}
			log.Printf("%s name=%s type=%s ip=%s port=%d txt=%v", "[-]", service.Name, service.Type, service.Address, service.Port, dm.parseTextRecord(service.Txt))
			// ItemRemove has no TXT records. Use the identifier captured from
			// ItemNew rather than the service name: checks and throttle entries
			// are keyed by identifier, not service name.
			dm.removeRemotePairingService(service.Name)
			// serviceName will change every mdns event, so we can't use serviceName to ignore duplicate
			dm.DeleteDeviceByServiceName(service.Name, model.DeviceConnectionRemote)
		case service, ok := <-sbRemoteManualPairing.AddChannel:
			if !ok {
				return errAvahiBrowserFreed
			}
			log.Printf("%s name=%s type=%s ip=%s port=%d txt=%v", "[+]", service.Name, service.Type, service.Address, service.Port, dm.parseTextRecord(service.Txt))

			service, err := server.ResolveService(service.Interface, service.Protocol, service.Name,
				service.Type, service.Domain, avahi.ProtoUnspec, 0)
			if err != nil {
				log.Err(err).Msgf("Failed to resolve service: name=%s type=%s", service.Name, service.Type)
				continue
			}

			name := service.Name
			if txtName := dm.parseTextRecordName(service.Txt); txtName != "" {
				name = txtName
			}
			identifier := dm.parseTextRecordIndentifier(service.Txt)
			if identifier == "" {
				log.Warnf("Remote manual pairing service missing identifier: name=%s ip=%s", service.Name, service.Address)
				continue
			}
			// use serviceName to ignore duplicate
			id := utils.Md5(service.Name)
			device := model.Device{
				ID:          id,
				Name:        name,
				ServiceName: service.Name,
				MacAddr:     "",
				IP:          service.Address,
				Port:        service.Port,
				UDID:        identifier,
				Connection:  model.DeviceConnectionRemote,
				Status:      model.Pairable,
				DiscoveryAt: time.Now(),
			}
			device.ParseDeviceClass()
			dm.SaveDevice(device)

		case service, ok := <-sbRemoteManualPairing.RemoveChannel:
			if !ok {
				return errAvahiBrowserFreed
			}
			log.Printf("%s name=%s type=%s ip=%s port=%d txt=%v", "[-]", service.Name, service.Type, service.Address, service.Port, dm.parseTextRecord(service.Txt))
			dm.DeleteDeviceByServiceName(service.Name, model.DeviceConnectionRemote)
		}
	}
}

func (dm *DeviceManager) Scan() {
	// TODO: AppleTV端删除连接后，本地自动删除已连接设备
}

// avahiBrowsers tracks the avahi browsers created for one operation on the
// shared server, so all of them are freed when the operation ends.
//
// A browser must be freed exactly once. Server.Close() frees every browser it
// still knows about, and freeing one afterwards closes an already-closed
// channel and panics. Browsers are created from per-service-type goroutines
// while the owning loop may return at any moment, so registration is
// serialized against teardown: Close marks the set closed first, so a browser
// created concurrently is either freed here or never freed at all.
type avahiBrowsers struct {
	server *avahi.Server

	mu       sync.Mutex
	closed   bool
	cleanups []func()
}

func newAvahiBrowsers(server *avahi.Server) *avahiBrowsers {
	return &avahiBrowsers{server: server}
}

// track registers a cleanup func. It returns false when the set is already
// closed, which means the underlying object has been freed by Close and must
// not be freed again.
func (b *avahiBrowsers) track(cleanup func()) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return false
	}
	b.cleanups = append(b.cleanups, cleanup)
	return true
}

// newServiceTypeBrowser creates a browser for all advertised service types
// (equivalent to `avahi-browse -a`).
func (b *avahiBrowsers) newServiceTypeBrowser(iface, protocol int32, domain string) *avahi.ServiceTypeBrowser {
	tb, err := b.server.ServiceTypeBrowserNew(iface, protocol, domain, 0)
	if err != nil {
		log.Err(err).Msg("ServiceTypeBrowserNew failed")
		return nil
	}

	if !b.track(func() { b.server.ServiceTypeBrowserFree(tb) }) {
		return nil
	}
	return tb
}

// newServiceBrowser creates a browser for one service type.
func (b *avahiBrowsers) newServiceBrowser(iface, protocol int32, serviceType string, domain string) *avahi.ServiceBrowser {
	sb, err := b.server.ServiceBrowserNew(iface, protocol, serviceType, domain, 0)
	if err != nil {
		log.Err(err).Msgf("ServiceBrowserNew failed for %s", serviceType)
		return nil
	}

	if !b.track(func() { b.server.ServiceBrowserFree(sb) }) {
		return nil
	}
	return sb
}

// Close frees every browser created for this operation. The shared server
// stays open. Close is idempotent.
func (b *avahiBrowsers) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}
	b.closed = true

	for _, cleanup := range b.cleanups {
		cleanup()
	}
	b.cleanups = nil
}

func (dm *DeviceManager) ScanServices(ctx context.Context, callback func(serviceType string, name string, host string, address string, port uint16, txt [][]byte)) error {
	server, err := avahiDaemon.get()
	if err != nil {
		return err
	}

	// Free every browser when the scan ends. Without it each scan leaves
	// its browsers registered on the shared server forever.
	browsers := newAvahiBrowsers(server)
	defer browsers.Close()

	// Use ServiceTypeBrowser to discover all advertised service types (equivalent to `avahi-browse -a`).
	typeBrowser := browsers.newServiceTypeBrowser(avahi.InterfaceUnspec, avahi.ProtoUnspec, mdnsServiceDomain)
	if typeBrowser == nil {
		return fmt.Errorf("service type browser new failed")
	}

	discoveredTypes := make(map[string]bool)

	for {
		select {
		case <-ctx.Done():
			return nil
		case entry, ok := <-typeBrowser.AddChannel:
			if !ok {
				return nil
			}

			serviceType := entry.Type
			if serviceType == "" || discoveredTypes[serviceType] {
				continue
			}

			discoveredTypes[serviceType] = true
			go dm.scanServiceTypeContinuous(ctx, browsers, server, entry.Interface, entry.Protocol, serviceType, entry.Domain, callback)
		case _, ok := <-typeBrowser.RemoveChannel:
			if !ok {
				return nil
			}
		}
	}
}

func (dm *DeviceManager) scanServiceTypeContinuous(ctx context.Context, browsers *avahiBrowsers, server *avahi.Server, iface, protocol int32, serviceType string, domain string, callback func(serviceType string, name string, host string, address string, port uint16, txt [][]byte)) {
	if serviceType == "" {
		return
	}
	if domain == "" {
		domain = mdnsServiceDomain
	}

	sb := browsers.newServiceBrowser(iface, protocol, serviceType, domain)
	if sb == nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case service, ok := <-sb.AddChannel:
			if !ok {
				return
			}
			resolved, err := server.ResolveService(service.Interface, service.Protocol, service.Name,
				service.Type, service.Domain, avahi.ProtoUnspec, 0)
			if err == nil {
				callback(resolved.Type, resolved.Name, resolved.Host, resolved.Address, resolved.Port, resolved.Txt)
			}
		case _, ok := <-sb.RemoveChannel:
			// Drain removals so the avahi signal dispatch goroutine never
			// blocks on an unread channel. ItemRemove is sent
			// unbuffered, so an undrained removal stalls signal delivery
			// for every browser on the server.
			if !ok {
				return
			}
		}
	}
}

// avahiServiceRef carries the fields needed to resolve a browsed service
// without keeping a reference to the channel element itself.
type avahiServiceRef struct {
	iface       int32
	protocol    int32
	name        string
	serviceType string
	domain      string
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

	server, err := avahiDaemon.get()
	if err != nil {
		return nil, err
	}

	browsers := newAvahiBrowsers(server)
	defer browsers.Close()

	sb := browsers.newServiceBrowser(avahi.InterfaceUnspec, avahi.ProtoUnspec, mdnsServiceRemotePairing, mdnsServiceDomain)
	if sb == nil {
		return nil, fmt.Errorf("service browser new failed")
	}

	devices := make([]model.Device, 0)
	deviceMap := make(map[string]bool)

	scanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// ResolveService is a synchronous D-Bus call without its own timeout.
	// Run it in a goroutine so a hanging avahi-daemon can never stall the
	// scan past scanCtx's deadline (which the gateway would report as 502).
	resolveService := func(svc avahiServiceRef) (avahi.Service, bool) {
		type result struct {
			service avahi.Service
			err     error
		}
		ch := make(chan result, 1)
		go func() {
			resolved, err := server.ResolveService(svc.iface, svc.protocol, svc.name,
				svc.serviceType, svc.domain, avahi.ProtoUnspec, 0)
			ch <- result{service: resolved, err: err}
		}()
		select {
		case <-scanCtx.Done():
			return avahi.Service{}, false
		case r := <-ch:
			if r.err != nil {
				return avahi.Service{}, false
			}
			return r.service, true
		}
	}

	for {
		select {
		case <-scanCtx.Done():
			return devices, nil
		case service, ok := <-sb.AddChannel:
			if !ok {
				return devices, nil
			}
			resolved, ok := resolveService(avahiServiceRef{
				iface:       service.Interface,
				protocol:    service.Protocol,
				name:        service.Name,
				serviceType: service.Type,
				domain:      service.Domain,
			})
			if !ok {
				if scanCtx.Err() != nil {
					return devices, nil
				}
				continue
			}

			// Avoid adding duplicates.
			if deviceMap[resolved.Address] {
				continue
			}
			deviceMap[resolved.Address] = true

			name := dm.parseName(resolved.Host)
			device := model.Device{
				ID:          utils.Md5(resolved.Name),
				Name:        name,
				ServiceName: service.Name,
				MacAddr:     "",
				IP:          resolved.Address,
				Port:        resolved.Port,
				Status:      model.Paired,
			}
			device.ParseDeviceClass()
			devices = append(devices, device)
		case _, ok := <-sb.RemoveChannel:
			// Drain removals so the avahi signal dispatch goroutine never
			// blocks on an unread channel while this one-shot scan runs.
			if !ok {
				return devices, nil
			}
		}
	}
}

func (dm *DeviceManager) parseTextRecord(txt [][]byte) map[string]string {
	result := make(map[string]string)
	for _, item := range txt {
		kv := strings.SplitN(string(item), "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}

func (dm *DeviceManager) parseTextRecordName(txt [][]byte) string {
	result := dm.parseTextRecord(txt)
	if name, ok := result["name"]; ok {
		return name
	}
	return ""
}

func (dm *DeviceManager) parseTextRecordIndentifier(txt [][]byte) string {
	result := dm.parseTextRecord(txt)
	if id, ok := result["identifier"]; ok {
		return id
	}
	return ""
}

func (dm *DeviceManager) parseTextRecordAuthTag(txt [][]byte) string {
	result := dm.parseTextRecord(txt)
	if id, ok := result["authTag"]; ok {
		return id
	}
	return ""
}
