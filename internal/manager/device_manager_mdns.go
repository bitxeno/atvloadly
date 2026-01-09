//go:build windows

package manager

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/utils"
	"github.com/grandcat/zeroconf"
)

const (
	discoverWaitTime = time.Second * 10

	mdnsService         = "_apple-mobdev2._tcp"
	mdnsServicePairable = "_apple-pairable._tcp"
	mdnsServiceDomain   = "local."
)

var ctx context.Context
var cancel context.CancelFunc

func (dm *DeviceManager) Start() {
	ctx, cancel = context.WithCancel(context.Background())

	// Discover all services on the network (e.g. _workstation._tcp)
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Err(err).Msg("Failed to initialize resolver: ")
	}

	entries := make(chan *zeroconf.ServiceEntry)
	entriesPairable := make(chan *zeroconf.ServiceEntry)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case entry := <-entries:
				if entry == nil {
					continue
				}
				serviceName := strings.Replace(entry.Instance, "\\@", "@", -1)
				log.Printf("Service discovered: name=%s type=%s ip=%v", serviceName, entry.Service, entry.AddrIPv4)

				macAddr := strings.Split(serviceName, "@")[0]
				host := dm.parseName(entry.HostName)
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
						Name:        host,
						ServiceName: serviceName,
						MacAddr:     macAddr,
						IP:          entry.AddrIPv4[0].String(),
						UDID:        udid,
						Status:      model.Paired,
					})
				}

			case entry := <-entriesPairable:
				if entry == nil {
					continue
				}
				serviceName := strings.Replace(entry.Instance, "\\@", "@", -1)
				log.Printf("Service discovered: name=%s type=%s ip=%v", serviceName, entry.Service, entry.AddrIPv4)

				// 添加可配对设备
				macAddr := strings.Split(serviceName, "@")[0]
				host := dm.parseName(entry.HostName)
				udid := fmt.Sprintf("fff%sfff", macAddr)
				dm.devices.Store(udid, model.Device{
					ID:          utils.Md5(udid),
					Name:        host,
					ServiceName: serviceName,
					MacAddr:     macAddr,
					IP:          entry.AddrIPv4[0].String(),
					UDID:        udid,
					Status:      model.Pairable,
				})
			}
		}
	}()

	if err := resolver.Browse(ctx, mdnsService, mdnsServiceDomain, entries); err != nil {
		log.Err(err).Msgf("Failed to browse: %s", mdnsService)
	}
	if err := resolver.Browse(ctx, mdnsServicePairable, mdnsServiceDomain, entriesPairable); err != nil {
		log.Err(err).Msgf("Failed to browse: %s", mdnsServicePairable)
	}

	log.Info("mDNS discovery started...")
	<-ctx.Done()
}

func (dm *DeviceManager) Scan() {
	cancel()
	<-ctx.Done()

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
	return nil
}

func (dm *DeviceManager) ScanWirelessDevices(ctx context.Context, timeout time.Duration) ([]model.Device, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, err
	}

	entries := make(chan *zeroconf.ServiceEntry)
	devices := []model.Device{}

	// 创建超时的context
	scanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 启动goroutine收集扫描结果
	done := make(chan struct{})
	go func() {
		defer close(done)
		for entry := range entries {
			serviceName := strings.Replace(entry.Instance, "\\@", "@", -1)
			macAddr := strings.Split(serviceName, "@")[0]
			host := dm.parseName(entry.HostName)

			var ip string
			if len(entry.AddrIPv4) > 0 {
				ip = entry.AddrIPv4[0].String()
			} else if len(entry.AddrIPv6) > 0 {
				ip = entry.AddrIPv6[0].String()
			}

			if ip != "" {
				device := model.Device{
					ID:          utils.Md5(serviceName),
					Name:        host,
					ServiceName: serviceName,
					MacAddr:     macAddr,
					IP:          ip,
					UDID:        serviceName,
					Status:      model.Pairable,
				}
				device.ParseDeviceClass()
				devices = append(devices, device)
			}
		}
	}()

	// 扫描mdnsService服务
	err = resolver.Browse(scanCtx, mdnsService, mdnsServiceDomain, entries)
	if err != nil && err != context.DeadlineExceeded {
		return devices, err
	}

	// 等待goroutine完成收集所有结果
	<-done

	return devices, nil
}
