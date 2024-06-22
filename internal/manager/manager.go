package manager

import "github.com/bitxeno/atvloadly/internal/model"

func StartDeviceManager() {
	go deviceManager.Start()
}

func GetDevices() ([]model.Device, error) {
	return deviceManager.GetDevices(), nil
}

func GetDeviceByID(id string) (*model.Device, bool) {
	return deviceManager.GetDeviceByID(id)
}

func GetDeviceMountImageInfo(udid string) (*model.UsbmuxdImage, error) {
	return deviceManager.GetMountImageInfo(udid)
}

func ReloadDevices() {
	deviceManager.ReloadDevices()
}

func ScanDevices() {
	deviceManager.Scan()
}

func RestartUsbmuxd() error {
	return deviceManager.RestartUsbmuxd()
}
