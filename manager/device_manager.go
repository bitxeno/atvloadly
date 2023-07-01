package manager

import (
	"log"
	"sync"

	"github.com/bitxeno/atvloadly/internal/utils"
	"github.com/bitxeno/atvloadly/model"
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
