//go:build linux

package manager

import (
	"context"
	"fmt"
	"strings"
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
)

// 需要依赖socket套接字：
// /var/run/dbus
// /var/run/avahi-daemon
func (dm *DeviceManager) Start() {
	dm.mu.Lock()
	dm.ctx, dm.cancel = context.WithCancel(context.Background())
	ctx := dm.ctx
	dm.mu.Unlock()

	conn, err := dbus.SystemBus()
	if err != nil {
		log.Printf("Cannot get system bus: %v", err)
		return
	}

	server, err := avahi.ServerNew(conn)
	if err != nil {
		log.Err(err).Msgf("Avahi new failed: ")
	}

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

	sbAppleMobdev, err := server.ServiceBrowserNew(avahi.InterfaceUnspec, avahi.ProtoUnspec, mdnsServiceAppleMobdev2, mdnsServiceDomain, 0)
	if err != nil {
		log.Err(err).Msgf("ServiceBrowserNew() failed: ")
	}

	sbRemotePairing, err := server.ServiceBrowserNew(avahi.InterfaceUnspec, avahi.ProtoUnspec, mdnsServiceRemotePairing, mdnsServiceDomain, 0)
	if err != nil {
		log.Err(err).Msgf("ServiceBrowserNew() failed: ")
	}

	sbRemoteManualPairing, err := server.ServiceBrowserNew(avahi.InterfaceUnspec, avahi.ProtoUnspec, mdnsServiceRemoteManualPairing, mdnsServiceDomain, 0)
	if err != nil {
		log.Err(err).Msgf("ServiceBrowserNew() failed: ")
	}

	log.Info("Avahi discovery started...")

	var service avahi.Service

	for {
		select {
		case <-ctx.Done():
			log.Info("Avahi discovery stopped")
			return
		case service = <-sbAppleMobdev.AddChannel:
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
		case service = <-sbAppleMobdev.RemoveChannel:
			log.Printf("%s name=%s type=%s ip=%s port=%d txt=%v", "[-]", service.Name, service.Type, service.Address, service.Port, dm.parseTextRecord(service.Txt))

			macAddr := strings.Split(service.Name, "@")[0]
			dm.DeleteDeviceByMacAddr(macAddr)
		case service = <-sbRemotePairing.AddChannel:
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

			if v, err := dm.CheckDevicePaired(identifier, authTag); err == nil && v != nil {
				log.Debugf("add rppairing device >> %v", v)
				if v.Name != "" {
					name = v.Name
				}
				device := model.Device{
					ID:          v.ID,
					Name:        name,
					ServiceName: service.Name,
					MacAddr:     "",
					IP:          service.Address,
					Port:        service.Port,
					UDID:        v.RemotePairingUDID,
					Connection:  model.DeviceConnectionRemote,
					Status:      model.Paired,
					DiscoveryAt: time.Now(),
				}

				if v.GetDeviceClass() != "" {
					device.DeviceClass = v.GetDeviceClass()
				} else {
					device.ParseDeviceClass()
				}

				// remove old lockdown connection device, avoid duplicate devices with both lockdown and remote connection
				dm.removeLockdownDevice(device.UDID)
				dm.SaveDevice(device)

				// Trigger device connection callback
				dm.onDeviceConnected(device)
			} else if err != nil {
				log.Debugf("Failed to check device pairing: name=%s ip=%s err=%s", service.Name, service.Address, err.Error())
			}
		case service = <-sbRemotePairing.RemoveChannel:
			log.Printf("%s name=%s type=%s ip=%s port=%d txt=%v", "[-]", service.Name, service.Type, service.Address, service.Port, dm.parseTextRecord(service.Txt))
			// serviceName will change every mdns event, so we can't use serviceName to ignore duplicate
			dm.DeleteDeviceByServiceName(service.Name, model.DeviceConnectionRemote)
		case service = <-sbRemoteManualPairing.AddChannel:
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

		case service = <-sbRemoteManualPairing.RemoveChannel:
			log.Printf("%s name=%s type=%s ip=%s port=%d txt=%v", "[-]", service.Name, service.Type, service.Address, service.Port, dm.parseTextRecord(service.Txt))
			dm.DeleteDeviceByServiceName(service.Name, model.DeviceConnectionRemote)
		}
	}
}

func (dm *DeviceManager) Scan() {
	// TODO: AppleTV端删除连接后，本地自动删除已连接设备
}

func (dm *DeviceManager) ScanServices(ctx context.Context, callback func(serviceType string, name string, host string, address string, port uint16, txt [][]byte)) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("cannot get system bus: %v", err)
	}

	server, err := avahi.ServerNew(conn)
	if err != nil {
		return fmt.Errorf("avahi new failed: %v", err)
	}

	// Use ServiceTypeBrowser to discover all advertised service types (equivalent to `avahi-browse -a`).
	typeBrowser, err := server.ServiceTypeBrowserNew(avahi.InterfaceUnspec, avahi.ProtoUnspec, mdnsServiceDomain, 0)
	if err != nil {
		return fmt.Errorf("service type browser new failed: %w", err)
	}
	defer server.ServiceTypeBrowserFree(typeBrowser)

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
			go dm.scanServiceTypeContinuous(ctx, server, entry.Interface, entry.Protocol, serviceType, entry.Domain, callback)
		case _, ok := <-typeBrowser.RemoveChannel:
			if !ok {
				return nil
			}
		}
	}
}

func (dm *DeviceManager) scanServiceTypeContinuous(ctx context.Context, server *avahi.Server, iface, protocol int32, serviceType string, domain string, callback func(serviceType string, name string, host string, address string, port uint16, txt [][]byte)) {
	if serviceType == "" {
		return
	}
	if domain == "" {
		domain = mdnsServiceDomain
	}

	sb, err := server.ServiceBrowserNew(iface, protocol, serviceType, domain, 0)
	if err != nil {
		log.Err(err).Msgf("ServiceBrowserNew failed for %s", serviceType)
		return
	}
	defer server.ServiceBrowserFree(sb)

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
		}
	}
}

func (dm *DeviceManager) ScanWirelessDevices(ctx context.Context, timeout time.Duration) ([]model.Device, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("cannot get system bus: %v", err)
	}

	server, err := avahi.ServerNew(conn)
	if err != nil {
		return nil, fmt.Errorf("avahi new failed: %v", err)
	}

	sb, err := server.ServiceBrowserNew(avahi.InterfaceUnspec, avahi.ProtoUnspec, mdnsServiceRemotePairing, mdnsServiceDomain, 0)
	if err != nil {
		return nil, fmt.Errorf("service browser new failed: %v", err)
	}

	devices := make([]model.Device, 0)
	deviceMap := make(map[string]bool)

	scanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case <-scanCtx.Done():
			return devices, nil
		case service := <-sb.AddChannel:
			resolved, err := server.ResolveService(service.Interface, service.Protocol, service.Name,
				service.Type, service.Domain, avahi.ProtoUnspec, 0)
			if err != nil {
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

func (dm *DeviceManager) removeLockdownDevice(udid string) {
	removeLockdownDevice(udid)
	dm.DeleteDeviceByUDID(udid)
}
