package manager

import (
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/model"
)

func StartDeviceManager() {
	go deviceManager.Start()
}

func GetDevices() ([]model.Device, error) {
	return deviceManager.GetDevices(), nil
}

func GetDeviceByID(id string) (*model.Device, bool) {
	return deviceManager.GetDeviceByID(id)
}

func AppendDeviceProductInfo(dev *model.Device) {
	deviceManager.AppendProductInfo(dev)
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

func CheckDeveloperMode(udid string) (bool, error) {
	return deviceManager.CheckDeveloperMode(udid)
}

func CheckAfcServiceStatus(udid string) error {
	return deviceManager.CheckAfcServiceStatus(udid)
}

func CheckDeviceStatus(udid string) error {
	return nil
}

func RestartUsbmuxd() error {
	return deviceManager.RestartUsbmuxd()
}

func ExecuteCommand(name string, args ...string) ([]byte, error) {
	timeout := 10 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = app.Config.Server.DataDir
	cmd.Env = os.Environ()
	return cmd.CombinedOutput()
}

func GetAppleAccounts() (*model.Accounts, error) {
	return accountManager.GetAccounts()
}

func DeleteAppleAccount(email string) error {
	return accountManager.DeleteAccount(email)
}
