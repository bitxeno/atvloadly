package manager

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/bitxeno/atvloadly/config"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/utils"
	"github.com/bitxeno/atvloadly/model"
	gidevice "github.com/electricbubble/gidevice"
)

var deviceManager = newDeviceManager()

type DeviceManager struct {
	devices sync.Map
}

func newDeviceManager() *DeviceManager {
	return &DeviceManager{}
}

func (dm *DeviceManager) GetDevices() []model.Device {
	devices := []model.Device{}
	dm.devices.Range(func(k, v interface{}) bool {
		devices = append(devices, v.(model.Device))
		return true
	})

	return devices
}

func (dm *DeviceManager) GetDeviceByID(id string) (*model.Device, bool) {
	devices := dm.GetDevices()
	for _, d := range devices {
		if d.ID == id {
			return &d, true
		}
	}

	return nil, false
}

func (dm *DeviceManager) GetDeviceByUDID(udid string) (*model.Device, bool) {
	if dev, ok := dm.devices.Load(udid); ok {
		return dev.(*model.Device), ok
	}

	return nil, false
}

func (dm *DeviceManager) ReloadDevices() {
	dm.devices.Range(func(k, v interface{}) bool {
		dev := v.(model.Device)
		if dev.Status == model.Pairable {
			// 检查是否已连接
			lockdownDevices, err := loadLockdownDevices()
			if err != nil {
				log.Println(err)
				return false
			}

			if lockdownDev, ok := lockdownDevices[dev.MacAddr]; ok {
				udid := lockdownDev.Name

				// 判断是否已存在连接状态
				if _, ok := dm.devices.Load(udid); ok {
					return false
				}

				// 添加新状态
				dev.ID = utils.Md5(udid)
				dev.Status = model.Paired
				dev.UDID = udid
				dm.devices.Store(udid, dev)
			}
		}

		return true
	})
}

// 获取DeveloperDiskImage绑定信息，install/screenshot等功能
// 都需要先绑定DeveloperDiskImage才有权限操作
func (dm *DeviceManager) GetMountImageInfo(udid string) (*model.UsbmuxdImage, error) {
	usbmux, err := gidevice.NewUsbmux()
	if err != nil {
		log.Err(err).Msg("Cannot get image signatures: ")
		return nil, err
	}

	devices, err := usbmux.Devices()
	if err != nil {
		log.Err(err).Msg("Cannot get image signatures: ")
		return nil, err
	}

	for _, dev := range devices {
		if dev.Properties().SerialNumber == udid {

			res, _ := dev.GetValue("", "")
			data, _ := json.Marshal(res)
			devInfo := new(model.UsbmuxdDevice)
			err := json.Unmarshal(data, devInfo)
			if err != nil {
				log.Err(err).Msg("Cannot get image signatures: ")
				return nil, err
			}

			imageSignatures, err := dev.Images()
			if err != nil {
				log.Err(err).Msg("Cannot get image signatures: ")
				return nil, err
			}

			imageInfo := model.NewUsbmuxdImage(*devInfo, config.Settings.DeveloperDiskImage.ImageSource)
			imageInfo.ImageMounted = len(imageSignatures) > 0
			return imageInfo, nil
		}
	}

	return nil, fmt.Errorf("Device pairing state not valid. Please try to pair again.")
}
