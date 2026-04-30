package manager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/exec"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/utils"
)

var deviceManager = newDeviceManager()

type DeviceManager struct {
	devices              sync.Map
	ctx                  context.Context
	cancel               context.CancelFunc
	mu                   sync.Mutex
	onDeviceConnected    func(device model.Device) // 设备连接时的回调函数
	onDeviceDisconnected func(device model.Device) // 设备断开时的回调函数
}

func newDeviceManager() *DeviceManager {
	return &DeviceManager{
		onDeviceConnected:    func(device model.Device) {},
		onDeviceDisconnected: func(device model.Device) {},
	}
}

func (dm *DeviceManager) GetDevices() []model.Device {
	devices := []model.Device{}
	dm.devices.Range(func(k, v any) bool {
		devices = append(devices, v.(model.Device))
		return true
	})

	// Sort devices: AppleTV type first, then by DeviceClass, then by Name
	sort.Slice(devices, func(i, j int) bool {
		classI := devices[i].DeviceClass
		classJ := devices[j].DeviceClass
		nameI := devices[i].Name
		nameJ := devices[j].Name

		// Detect AppleTV by DeviceClass or fallback to name if empty
		lowerClassI := strings.ToLower(classI)
		lowerClassJ := strings.ToLower(classJ)

		isAppleTVI := strings.Contains(lowerClassI, "appletv")
		isAppleTVJ := strings.Contains(lowerClassJ, "appletv")

		// AppleTV type has highest priority
		if isAppleTVI != isAppleTVJ {
			return isAppleTVI
		}

		// If classes differ, sort by DeviceClass string
		if classI != classJ {
			return classI < classJ
		}

		// Finally sort by name
		return nameI < nameJ
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
	devices := dm.GetDevices()
	for _, d := range devices {
		if d.UDID == udid {
			return &d, true
		}
	}

	return nil, false
}

func (dm *DeviceManager) GetDeviceInfo(dev *model.Device) (*model.DeviceInfo, error) {
	cmd := exec.Command("plumesign", "device-info", "-u", dev.UDID).WithTimeout(5 * time.Second)
	if dev.Connection == model.DeviceConnectionRemote {
		cmd = exec.Command("plumesign", "device-info", "--ip", dev.IP, "--port", fmt.Sprintf("%d", dev.Port), "-u", dev.UDID).WithTimeout(5 * time.Second)
	}

	data, err := cmd.CombinedOutput()
	if err != nil {
		log.Err(err).Msgf("Error getting device info for %s (%s): %s", dev.Name, dev.UDID, string(data))
		return nil, fmt.Errorf("%s%s", string(data), err.Error())
	}
	output := string(data)
	lines := strings.Split(string(output), "\n")

	devInfo := &model.DeviceInfo{UniqueDeviceID: dev.UDID}
	for _, line := range lines {
		var parts = strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "UniqueDeviceID":
			devInfo.UniqueDeviceID = value
		case "ProductName":
			devInfo.ProductName = value
		case "ProductType":
			devInfo.ProductType = value
		case "ProductVersion":
			devInfo.ProductVersion = value
		case "DeviceClass":
			devInfo.DeviceClass = value
		case "DeviceName":
			devInfo.DeviceName = value
		case "WiFiAddress":
			devInfo.WiFiAddress = value
		case "SerialNumber":
			devInfo.SerialNumber = value
		case "PersonalizedImageMounted":
			devInfo.PersonalizedImageMounted = value == "true"
		case "DeveloperModeStatus":
			devInfo.DeveloperModeStatus = value == "true"
		}
	}
	return devInfo, nil
}

func (dm *DeviceManager) AppendProductInfo(dev *model.Device, devInfo model.DeviceInfo) {
	if dev.Name != devInfo.DeviceName ||
		dev.ProductVersion != devInfo.ProductVersion ||
		dev.DeviceClass != devInfo.DeviceClass ||
		dev.PersonalizedImageMounted != devInfo.PersonalizedImageMounted ||
		dev.DeveloperModeStatus != devInfo.DeveloperModeStatus {

		dev.Name = devInfo.DeviceName
		dev.ProductType = devInfo.ProductType
		dev.ProductVersion = devInfo.ProductVersion
		dev.DeviceClass = devInfo.DeviceClass
		dev.PersonalizedImageMounted = devInfo.PersonalizedImageMounted
		dev.DeveloperModeStatus = devInfo.DeveloperModeStatus

		dm.SaveDevice(*dev)
	}
}

func (dm *DeviceManager) SaveDevice(dev model.Device) {
	dm.devices.Store(dev.ID, dev)
}

func (dm *DeviceManager) DeleteDevice(id string) {
	dm.devices.Delete(id)
}

func (dm *DeviceManager) DeleteDeviceByUDID(udid string) {
	dm.devices.Range(func(k, v any) bool {
		if v.(model.Device).UDID == udid {
			dm.devices.Delete(k)
			return false
		}
		return true
	})
}

func (dm *DeviceManager) DeleteDeviceByMacAddr(macAddr string) {
	dm.devices.Range(func(k, v any) bool {
		if v.(model.Device).MacAddr == macAddr {
			dm.devices.Delete(k)
			return false
		}
		return true
	})
}

func (dm *DeviceManager) HasCheckedDevice(ip string, port uint16, name string) bool {
	hasChecked := false
	dm.devices.Range(func(k, v any) bool {
		dev := v.(model.Device)
		if dev.IP == ip && dev.Port == port && dev.Status == model.Paired {
			hasChecked = true
			return false
		}
		return true
	})
	return hasChecked
}

func (dm *DeviceManager) DeleteDeviceByServiceName(serviceName string, conection model.DeviceConnection) {
	dm.devices.Range(func(k, v any) bool {
		if v.(model.Device).Connection == conection && v.(model.Device).ServiceName == serviceName {
			dm.devices.Delete(k)
			return false
		}
		return true
	})
}

func (dm *DeviceManager) ReloadDevices() {
	dm.devices.Range(func(k, v any) bool {
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

func (dm *DeviceManager) CheckAfcServiceStatus(dev *model.Device) error {
	cmd := exec.Command("plumesign", "check", "afc", "--udid", dev.UDID).WithTimeout(10 * time.Second)
	if dev.Connection == model.DeviceConnectionRemote {
		cmd = exec.Command("plumesign", "check", "afc", "--ip", dev.IP, "--port", fmt.Sprintf("%d", dev.Port), "--udid", dev.UDID).WithTimeout(10 * time.Second)
	}

	data, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	output := string(data)
	if strings.Contains(output, "Error") {
		return fmt.Errorf("%s", output)
	}

	if !strings.Contains(output, "SUCCESS") {
		return fmt.Errorf("%s", output)
	}

	return nil
}

func (dm *DeviceManager) CheckDevicePaired(identifier string, authTag string) (*model.RemoteDevice, error) {
	if !utils.ExistFiles(app.RemotePairingDir(), "*.plist") {
		return nil, nil
	}

	cmd := exec.Command("plumesign", "check", "find-pairing", "--identifier", identifier, "--auth-tag", authTag).WithTimeout(10 * time.Second)

	data, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile("UDID: `([^`]+)`")
	matches := re.FindStringSubmatch(string(data))
	if len(matches) > 1 {
		udid := matches[1]

		remoteDev := model.RemoteDevice{
			ID:                utils.Md5(udid),
			RemotePairingUDID: udid,
		}

		peerDevicePath := filepath.Join(app.RemotePairingDir(), fmt.Sprintf("%s.json", udid))
		if utils.Exists(peerDevicePath) {
			if data, err := os.ReadFile(peerDevicePath); err == nil {
				if err := utils.FromJSON(data, &remoteDev); err == nil {
					remoteDev.ID = utils.Md5(remoteDev.RemotePairingUDID)
				}
			}
		}

		return &remoteDev, nil
	}

	return nil, fmt.Errorf("failed to parse device information")
}

func (dm *DeviceManager) parseName(host string) string {
	name := strings.TrimSuffix(host, ".")
	name = strings.TrimSuffix(name, ".local")
	return name
}

// SetOnDeviceConnected Set the callback function for device connection
func (dm *DeviceManager) SetOnDeviceConnected(callback func(device model.Device)) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if callback != nil {
		dm.onDeviceConnected = callback
	}
}

// SetOnDeviceDisconnected Set the callback function for device disconnection
func (dm *DeviceManager) SetOnDeviceDisconnected(callback func(device model.Device)) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if callback != nil {
		dm.onDeviceDisconnected = callback
	}
}

// Stop Stop the device manager
func (dm *DeviceManager) Stop() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.cancel != nil {
		dm.cancel()
		dm.cancel = nil
		dm.ctx = nil
		log.Info("Device manager stopped")
	}
}

// Exported functions for external package usage

// SetDeviceConnectedCallback Set the callback function for device connection (exported function)
func SetDeviceConnectedCallback(callback func(device model.Device)) {
	deviceManager.SetOnDeviceConnected(callback)
}

// SetDeviceDisconnectedCallback Set the callback function for device disconnection (exported function)
func SetDeviceDisconnectedCallback(callback func(device model.Device)) {
	deviceManager.SetOnDeviceDisconnected(callback)
}
