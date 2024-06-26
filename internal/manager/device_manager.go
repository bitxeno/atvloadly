package manager

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/utils"
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

// Get AppleTV mounted information of DeveloperDiskImage
// install/screenshot function need mounted DeveloperDiskImage to operate.
func (dm *DeviceManager) GetMountImageInfo(udid string) (*model.UsbmuxdImage, error) {
	devInfo, err := dm.GetUsbmuxdDeviceInfo(udid)
	if err != nil {
		log.Err(err).Msg("Cannot get device info: ")
		return nil, err
	}

	imageInfo := model.NewUsbmuxdImage(*devInfo, app.Config.App.DeveloperDiskImage.ImageSource)
	imageMounted, err := dm.CheckHasMountImage(udid)
	if err == nil {
		imageInfo.ImageMounted = imageMounted
		return imageInfo, nil
	}

	// AppleTV system has reboot, need restart usbmuxd to fix lookup_image error
	if strings.Contains(err.Error(), "lookup_image returned -256") {
		if err = dm.RestartUsbmuxd(); err == nil {
			time.Sleep(5 * time.Second)
			if imageMounted, err = dm.CheckHasMountImage(udid); err == nil {
				imageInfo.ImageMounted = imageMounted
				return imageInfo, nil
			}
		}
	}

	log.Err(err).Msg("Cannot get image signature: ")
	return nil, err
}

func (dm *DeviceManager) GetUsbmuxdDeviceInfo(udid string) (*model.UsbmuxdDevice, error) {
	cmd := exec.Command("ideviceinfo", "-u", udid, "-n")

	data, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s%s", string(data), err.Error())
	}

	device := new(model.UsbmuxdDevice)
	output := string(data)
	lines := strings.Split(output, "\n")
	for _, v := range lines {
		arr := strings.Split(v, ":")
		if len(arr) == 2 {
			switch strings.TrimSpace(arr[0]) {
			case "ProductVersion":
				device.ProductVersion = strings.TrimSpace(arr[1])
			case "ProductName":
				device.ProductName = strings.TrimSpace(arr[1])
			case "DeviceName":
				device.DeviceName = strings.TrimSpace(arr[1])
			}
		}
	}

	return device, nil
}

func (dm *DeviceManager) CheckHasMountImage(udid string) (bool, error) {
	cmd := exec.Command("ideviceimagemounter", "-u", udid, "-n", "-l")

	data, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("%s%s", string(data), err.Error())
	}

	output := string(data)
	if strings.Contains(output, "ERROR") {
		return false, fmt.Errorf("%s", output)
	}

	return strings.Contains(output, "ImageSignature") && !strings.Contains(output, "ImageSignature[0]"), nil
}

func (dm *DeviceManager) CheckAfcServiceStatus(udid string) error {
	cmd := exec.Command("sideloader", "check", "afc", "--nocolor", "--udid", udid)

	data, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s%s", string(data), err.Error())
	}

	output := string(data)
	if strings.Contains(output, "ERROR") {
		return fmt.Errorf("%s", output)
	}

	if !strings.Contains(output, "SUCCESS") {
		return fmt.Errorf("%s", output)
	}

	return nil
}

func (dm *DeviceManager) RestartUsbmuxd() error {
	cmd := exec.Command("/etc/init.d/usbmuxd", "restart")
	data, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s%s", string(data), err.Error())
	}

	return nil
}
