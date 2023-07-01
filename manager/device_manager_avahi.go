//go:build linux

package manager

import (
	"fmt"
	"strings"

	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/utils"
	"github.com/bitxeno/atvloadly/model"
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
	log.Println("GetHostName()", host)

	fqdn, err := server.GetHostNameFqdn()
	if err != nil {
		log.Err(err).Msgf("GetHostNameFqdn() failed: ")
	}
	log.Println("GetHostNameFqdn()", fqdn)

	s, err := server.GetAlternativeHostName(host)
	if err != nil {
		log.Err(err).Msgf("GetAlternativeHostName() failed: ")
	}
	log.Println("GetAlternativeHostName()", s)

	i, err := server.GetAPIVersion()
	if err != nil {
		log.Err(err).Msgf("GetAPIVersion() failed: ")
	}
	log.Println("GetAPIVersion()", i)

	hn, err := server.ResolveHostName(avahi.InterfaceUnspec, avahi.ProtoUnspec, fqdn, avahi.ProtoUnspec, 0)
	if err != nil {
		log.Err(err).Msgf("ResolveHostName() failed: ")
	}
	log.Println("ResolveHostName:", hn)

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
			log.Println("ServiceBrowser ADD: ", service)

			service, err := server.ResolveService(service.Interface, service.Protocol, service.Name,
				service.Type, service.Domain, avahi.ProtoUnspec, 0)
			if err == nil {
				log.Println(" RESOLVED >>", service.Address)

				macAddr := strings.Split(service.Name, "@")[0]
				name := strings.TrimSuffix(service.Host, ".local")
				// 检查是否已连接
				lockdownDevices, err := loadLockdownDevices()
				if err != nil {
					log.Println(err)
					continue
				}

				// 添加已连接设备，TODO：handshake检测是否可真实连接
				if lockdownDev, ok := lockdownDevices[macAddr]; ok {
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
			log.Println("ServiceBrowser REMOVE: ", service)
		case service = <-sbPairable.AddChannel:
			log.Println("ServiceBrowser ADD: ", service)

			service, err := server.ResolveService(service.Interface, service.Protocol, service.Name,
				service.Type, service.Domain, avahi.ProtoUnspec, 0)
			if err == nil {
				log.Println(" RESOLVED >>", service.Address)

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
			log.Println("ServiceBrowser REMOVE: ", service)
			macAddr := strings.Split(service.Name, "@")[0]
			udid := fmt.Sprintf("fff%sfff", macAddr)
			dm.devices.Delete(udid)
		}
	}
}

func (dm *DeviceManager) Scan() {
	// TODO: AppleTV端删除连接后，本地自动删除已连接设备
}
