package manager

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/model"
)

func ScanServices(ctx context.Context, callback func(serviceType string, name string, host string, address string, port uint16, txt [][]byte)) error {
	return deviceManager.ScanServices(ctx, callback)
}

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
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Parse error output
		var found []string
		for _, line := range strings.Split(string(output), "\n") {
			s := strings.ToLower(strings.TrimSpace(line))
			if strings.HasPrefix(s, "error:") {
				found = append(found, s)
			}
		}
		if len(found) > 0 {
			return output, errors.New(strings.Join(found, "\n"))
		}
	}
	return output, err
}

func GetAppleAccounts() (*model.Accounts, error) {
	return accountManager.GetAccounts()
}

func LogoutAppleAccount(email string) error {
	return accountManager.LogoutAccount(email)
}

func GetAccountDevices(email string) ([]model.AccountDevice, error) {
	return accountManager.GetAccountDevices(email)
}

func DeleteAccountDevice(email, deviceID string) error {
	return accountManager.DeleteAccountDevice(email, deviceID)
}

func GetCertificates(email string) ([]model.Certificate, error) {
	return certificateManager.GetCertificates(email)
}

func RevokeCertificate(email string, serialNumber string) error {
	return certificateManager.RevokeCertificate(email, serialNumber)
}
