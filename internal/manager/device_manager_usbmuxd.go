//go:build darwin

package manager

import (
	"context"
	"encoding/json"
	"net/netip"
	"strings"
	"time"

	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/utils"
	gidevice "github.com/electricbubble/gidevice"
	"github.com/grandcat/zeroconf"
)

const (
	mdnsService       = "_apple-mobdev2._tcp"
	mdnsServiceDomain = "local"
)

var usbmux gidevice.Usbmux

func (dm *DeviceManager) Start() {
	dm.mu.Lock()
	dm.ctx, dm.cancel = context.WithCancel(context.Background())
	ctx := dm.ctx
	dm.mu.Unlock()

	umx, err := gidevice.NewUsbmux()
	if err != nil {
		log.Err(err).Msg("Cannot connect to usbmuxd")
		return
	}
	usbmux = umx

	t := time.NewTimer(0)
	defer t.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Info("Device scanner stopped")
				return
			case <-t.C:
				dm.Scan()
				t.Reset(10 * time.Second)
			}
		}
	}()

	<-ctx.Done()
}

func (dm *DeviceManager) Scan() {
	devices, err := usbmux.Devices()
	if err != nil {
		log.Err(err).Msg("Cannot get devices")
		return
	}

	keepConnectedDevices := make(map[string]bool)
	for _, d := range devices {
		if d.Properties().ConnectionType != "Network" {
			continue
		}

		uuid := d.Properties().SerialNumber
		keepConnectedDevices[uuid] = true
		macAddr := strings.Split(d.Properties().EscapedFullServiceName, "@")[0]

		device := model.Device{
			ID:          utils.Md5(uuid),
			Name:        "AppleTV",
			ServiceName: d.Properties().EscapedFullServiceName,
			MacAddr:     macAddr,
			IP:          dm.parseNetworkAddress(d.Properties().NetworkAddress),
			UDID:        uuid,
		}
		if strings.Contains(uuid, ":") {
			device.Status = model.Pairable
		} else {
			res, _ := d.GetValue("", "")
			data, _ := json.Marshal(res)
			devInfo := new(model.UsbmuxdDevice)
			if err := json.Unmarshal(data, devInfo); err == nil {
				device.Name = devInfo.DeviceName
				device.ProductType = devInfo.ProductType
				device.ProductVersion = devInfo.ProductVersion
				device.DeviceClass = devInfo.DeviceClass
			}
			device.Status = model.Paired
			device.ParseDeviceClass()
		}

		dm.devices.Store(uuid, device)
	}

	// Delete non-existent devices
	dm.devices.Range(func(key, value any) bool {
		uuid := key.(string)
		if !keepConnectedDevices[uuid] {
			dm.devices.Delete(uuid)
		}
		return true
	})
}

// data布局：https://github.com/jkcoxson/netmuxd/blob/48494cf6e264bed4e6e1bfa8015767f515ac9ca3/src/devices.rs#L303
func (dm *DeviceManager) parseNetworkAddress(networkAddress []byte) string {
	networkFamily := networkAddress[0]

	if networkFamily == 16 {
		// ipv4
		if ip, ok := netip.AddrFromSlice(networkAddress[4:8]); ok {
			return ip.String()
		}
	}

	if networkFamily == 28 {
		// ipv6
		if ip, ok := netip.AddrFromSlice(networkAddress[16:32]); ok {
			return ip.String()
		}
	}

	return ""

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
