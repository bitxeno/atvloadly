package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
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

func GetDeviceDetail(id string) (*model.Device, bool) {
	device, found := deviceManager.GetDeviceByID(id)
	if found {
		if device.Connection == model.DeviceConnectionLockdown {
			if devInfo, err := deviceManager.GetDeviceInfo(device); err == nil {
				deviceManager.AppendProductInfo(device, *devInfo)
			}
		}
	}
	return device, found
}

func GetDeviceByID(id string) (*model.Device, bool) {
	return deviceManager.GetDeviceByID(id)
}

func GetDeviceByUDID(udid string) (*model.Device, bool) {
	return deviceManager.GetDeviceByUDID(udid)
}

// PrepareManualDevice resolves the dynamic tunnel port of a manual device so
// that installs, refreshes and screenshots can reach it over RSD. It is a
// no-op for mDNS-discovered devices.
func PrepareManualDevice(ctx context.Context, dev *model.Device) error {
	return deviceManager.PrepareManualDevice(ctx, dev)
}

// ManualPortNeedsResolve reports whether the next PrepareManualDevice call will
// probe for a manual device's tunnel port (missing or older than the cache
// TTL). Used to show a probe prompt before the potentially slow discovery.
func ManualPortNeedsResolve(dev *model.Device) bool {
	if dev == nil || !dev.Manual || dev.IP == "" {
		return false
	}
	return dev.Port == 0 || time.Since(dev.PortResolvedAt) >= manualPortTTL
}

// WarmManualDevicePort resolves a manual device's tunnel port in the
// background after its pairing completes, so the first install does not pay
// for the full-range scan. Failures are logged only; the install page
// resolves the port on demand.
func WarmManualDevicePort(ip string) {
	deviceManager.WarmManualDevicePort(ip)
}

// GetDeviceInfo queries the device for its lockdown values, including the
// hardware UniqueDeviceID.
func GetDeviceInfo(dev *model.Device) (*model.DeviceInfo, error) {
	return deviceManager.GetDeviceInfo(dev)
}

func ReloadDevices() {
	deviceManager.ReloadDevices()
}

// ReloadManualDevices rebuilds the manual-device list from the pairing files.
func ReloadManualDevices() {
	deviceManager.ReloadManualDevices()
}

// DeleteManualDevice removes a manually added device: its pairing files
// (<udid>.json and <udid>.plist) and its in-memory entry.
func DeleteManualDevice(udid string) error {
	if strings.TrimSpace(udid) == "" {
		return fmt.Errorf("udid is required")
	}
	// Both files belong to the same pairing record: delete them together.
	for _, ext := range []string{".json", ".plist"} {
		path := filepath.Join(app.RemotePairingDir(), udid+ext)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	deviceManager.RemoveManualDevice(utils.Md5(udid))
	return nil
}

func ScanDevices() {
	deviceManager.Scan()
}

func ScanWirelessDevices(ctx context.Context, timeout time.Duration) ([]model.Device, error) {
	return deviceManager.ScanWirelessDevices(ctx, timeout)
}

func CheckAfcServiceStatus(udid string) error {
	device, found := deviceManager.GetDeviceByUDID(udid)
	if !found {
		return fmt.Errorf("device not found: %s", udid)
	}
	// Manual devices need their tunnel port resolved before the AFC check.
	if err := PrepareManualDevice(context.Background(), device); err != nil {
		return err
	}
	return deviceManager.CheckAfcServiceStatus(device)
}

func CheckDeviceStatus(udid string) error {
	return nil
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

func ImportCertificate(email, password string, p12 []byte) error {
	return certificateManager.ImportCertificate(email, password, p12)
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

func Usbmuxd() *UsbmuxdManager {
	return usbmuxdManager
}
