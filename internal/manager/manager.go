package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/utils"
)

func ScanServices(ctx context.Context, callback func(serviceType string, name string, host string, address string, port uint16, txt [][]byte)) error {
	return deviceManager.ScanServices(ctx, callback)
}

func StartDeviceManager() {
	// 如果之前已启动，先停止
	StopDeviceManager()
	go deviceManager.Start()
}

func StopDeviceManager() {
	deviceManager.Stop()
}

func GetDevices() ([]model.Device, error) {
	return deviceManager.GetDevices(), nil
}

func GetDeviceByID(id string) (*model.Device, bool) {
	device, found := deviceManager.GetDeviceByID(id)
	if found {
		if devInfo, err := GetDeviceInfo(device.UDID); err == nil {
			deviceManager.AppendProductInfo(device, *devInfo)
		}
	}
	return device, found
}

func GetDeviceByUDID(udid string) (*model.Device, bool) {
	device, found := deviceManager.GetDeviceByUDID(udid)
	if found {
		if devInfo, err := GetDeviceInfo(device.UDID); err == nil {
			deviceManager.AppendProductInfo(device, *devInfo)
		}
	}
	return device, found
}

func GetDeviceInfo(udid string) (*model.DeviceInfo, error) {
	return deviceManager.GetDeviceInfo(udid)
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

func ScanWirelessDevices(ctx context.Context, timeout time.Duration) ([]model.Device, error) {
	return deviceManager.ScanWirelessDevices(ctx, timeout)
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

func ExportCertificate(email, password string) ([]byte, error) {
	tempDir := os.TempDir()
	fileName := fmt.Sprintf("cert_%d.p12", time.Now().Unix())
	tempFile := filepath.Join(tempDir, fileName)

	if _, err := certificateManager.ExportCertificate(email, password, tempFile); err != nil {
		return nil, err
	}

	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		return nil, errors.New("certificate file not generated")
	}

	content, err := os.ReadFile(tempFile)
	if err != nil {
		return nil, errors.New("failed to read generated file")
	}
	_ = os.Remove(tempFile)
	return content, nil
}

func ImportCertificate(email, password, path string) error {
	return certificateManager.ImportCertificate(email, password, path)
}

func GetRunEnvs() []string {
	envs := []string{}
	if app.Settings.Network.ProxyEnabled {
		if app.Settings.Network.HTTPProxy != "" {
			envs = append(envs, fmt.Sprintf("HTTP_PROXY=%s", app.Settings.Network.HTTPProxy))
			envs = append(envs, fmt.Sprintf("http_proxy=%s", app.Settings.Network.HTTPProxy))
		}
		if app.Settings.Network.HTTPSProxy != "" {
			envs = append(envs, fmt.Sprintf("HTTPS_PROXY=%s", app.Settings.Network.HTTPSProxy))
			envs = append(envs, fmt.Sprintf("https_proxy=%s", app.Settings.Network.HTTPSProxy))
		}
	}
	return utils.MergeEnvs(os.Environ(), envs)
}
