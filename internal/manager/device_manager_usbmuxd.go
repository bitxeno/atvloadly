//go:build darwin

package manager

import (
	"context"
	"encoding/json"
	"log"
	"net/netip"
	"strings"
	"time"

	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/utils"
	gidevice "github.com/electricbubble/gidevice"
)

var usbmux gidevice.Usbmux

func (dm *DeviceManager) Start() {
	umx, err := gidevice.NewUsbmux()
	if err != nil {
		log.Panicf("Cannot connect to usbmuxd: %v", err)
		return
	}
	usbmux = umx

	t := time.NewTimer(0)
	go func() {
		for {
			<-t.C
			dm.Scan()

			t.Reset(10 * time.Second)
		}
	}()
}

func (dm *DeviceManager) Scan() {
	devices, err := usbmux.Devices()
	if err != nil {
		log.Printf("Cannot get devices: %v", err)
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
