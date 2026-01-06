//go:build linux

package manager

import (
	"context"
	"fmt"
	"strings"

	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/utils"
	"github.com/godbus/dbus/v5"
	"github.com/holoplot/go-avahi"
)

const (
	mdnsService         = "_apple-mobdev2._tcp"
	mdnsServicePairable = "_apple-pairable._tcp"
	mdnsServiceDomain   = "local"
)

// 需要依赖socket套接字：
// /var/run/dbus
// /var/run/avahi-daemon
func (dm *DeviceManager) Start() {
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
	log.Tracef("GetHostName(): %s", host)

	fqdn, err := server.GetHostNameFqdn()
	if err != nil {
		log.Err(err).Msgf("GetHostNameFqdn() failed: ")
	}
	log.Tracef("GetHostNameFqdn(): %s", fqdn)

	s, err := server.GetAlternativeHostName(host)
	if err != nil {
		log.Err(err).Msgf("GetAlternativeHostName() failed: ")
	}
	log.Tracef("GetAlternativeHostName(): %s", s)

	i, err := server.GetAPIVersion()
	if err != nil {
		log.Err(err).Msgf("GetAPIVersion() failed: ")
	}
	log.Tracef("GetAPIVersion(): %v", i)

	hn, err := server.ResolveHostName(avahi.InterfaceUnspec, avahi.ProtoUnspec, fqdn, avahi.ProtoUnspec, 0)
	if err != nil {
		log.Err(err).Msgf("ResolveHostName() failed: ")
	}
	log.Tracef("ResolveHostName: %v", hn)

	sb, err := server.ServiceBrowserNew(avahi.InterfaceUnspec, avahi.ProtoUnspec, mdnsService, mdnsServiceDomain, 0)
	if err != nil {
		log.Err(err).Msgf("ServiceBrowserNew() failed: ")
	}

	sbPairable, err := server.ServiceBrowserNew(avahi.InterfaceUnspec, avahi.ProtoUnspec, mdnsServicePairable, mdnsServiceDomain, 0)
	if err != nil {
		log.Err(err).Msgf("ServiceBrowserNew() failed: ")
	}

	log.Info("mDNS discovery started...")

	var service avahi.Service

	for {
		select {
		case service = <-sb.AddChannel:
			log.Tracef("ServiceBrowser ADD: %v", service)

			service, err := server.ResolveService(service.Interface, service.Protocol, service.Name,
				service.Type, service.Domain, avahi.ProtoUnspec, 0)
			if err == nil {
				log.Tracef(" RESOLVED >> %s", service.Address)

				macAddr := strings.Split(service.Name, "@")[0]
				name := strings.TrimSuffix(service.Host, ".local")
				// 检查是否已连接
				lockdownDevices, err := loadLockdownDevices()
				if err != nil {
					log.Err(err).Msg("loadLockdownDevices error: ")
					continue
				}
				log.Tracef("lockdown devices count >> %v", len(lockdownDevices))

				// 添加已连接设备，TODO：handshake检测是否可真实连接
				if lockdownDev, ok := lockdownDevices[macAddr]; ok {
					log.Tracef("add lockdown device >> %v", lockdownDev)
					udid := lockdownDev.Name
					dm.devices.Store(udid, model.Device{
						ID:          utils.Md5(udid),
						Name:        name,
						ServiceName: service.Name,
						MacAddr:     macAddr,
						IP:          service.Address,
						UDID:        udid,
						Status:      model.Paired,
					})
				}
			}
		case service = <-sb.RemoveChannel:
			log.Tracef("ServiceBrowser REMOVE: %v", service)
		case service = <-sbPairable.AddChannel:
			log.Tracef("ServiceBrowser ADD: %v", service)

			service, err := server.ResolveService(service.Interface, service.Protocol, service.Name,
				service.Type, service.Domain, avahi.ProtoUnspec, 0)
			if err == nil {
				log.Tracef(" RESOLVED >> %s", service.Address)

				// 添加可配对设备
				macAddr := strings.Split(service.Name, "@")[0]
				name := strings.TrimSuffix(service.Host, ".local")
				udid := fmt.Sprintf("fff%sfff", macAddr)
				dm.devices.Store(udid, model.Device{
					ID:          utils.Md5(udid),
					Name:        name,
					ServiceName: service.Name,
					MacAddr:     macAddr,
					IP:          service.Address,
					UDID:        udid,
					Status:      model.Pairable,
				})

			}

		case service = <-sbPairable.RemoveChannel:
			log.Tracef("ServiceBrowser REMOVE: %v", service)
			macAddr := strings.Split(service.Name, "@")[0]
			udid := fmt.Sprintf("fff%sfff", macAddr)
			dm.devices.Delete(udid)
		}
	}
}

func (dm *DeviceManager) Scan() {
	// TODO: AppleTV端删除连接后，本地自动删除已连接设备
}

func (dm *DeviceManager) ScanServices(ctx context.Context, callback func(serviceType string, name string, host string, address string, port uint16, txt [][]byte)) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("Cannot get system bus: %v", err)
	}

	server, err := avahi.ServerNew(conn)
	if err != nil {
		return fmt.Errorf("Avahi new failed: %v", err)
	}

	// Browse all service types
	stb, err := server.ServiceTypeBrowserNew(avahi.InterfaceUnspec, avahi.ProtoUnspec, mdnsServiceDomain, 0)
	if err != nil {
		return fmt.Errorf("ServiceTypeBrowserNew failed: %w", err)
	}

	discoveredTypes := make(map[string]bool)

	// Goroutine to handle type discovery and spawn service browsers
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-stb.AddChannel:
				if !discoveredTypes[t.Type] {
					discoveredTypes[t.Type] = true
					go dm.scanServiceTypeContinuous(ctx, server, t.Type, callback)
				}
			}
		}
	}()

	<-ctx.Done()
	return nil
}

func (dm *DeviceManager) scanServiceTypeContinuous(ctx context.Context, server *avahi.Server, serviceType string, callback func(serviceType string, name string, host string, address string, port uint16, txt [][]byte)) {
	sb, err := server.ServiceBrowserNew(avahi.InterfaceUnspec, avahi.ProtoUnspec, serviceType, mdnsServiceDomain, 0)
	if err != nil {
		log.Err(err).Msgf("ServiceBrowserNew failed for %s", serviceType)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case service := <-sb.AddChannel:
			resolved, err := server.ResolveService(service.Interface, service.Protocol, service.Name,
				service.Type, service.Domain, avahi.ProtoUnspec, 0)
			if err == nil {
				callback(serviceType, resolved.Name, resolved.Host, resolved.Address, resolved.Port, resolved.Txt)
			}
		}
	}
}
