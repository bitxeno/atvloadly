package manager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
)

var accountManager = newAccountManager()

type AccountManager struct{}

func newAccountManager() *AccountManager {
	return &AccountManager{}
}

// GetAccounts reads account file and returns simplified account info.
// The accounts.json may be an array or an object; normalize both to a slice.
func (am *AccountManager) GetAccounts() (*model.Accounts, error) {
	path := filepath.Join(app.SideloadDataDir(), "accounts.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &model.Accounts{}, nil
		}
		return nil, err
	}

	// 1) try as array of accounts
	var a model.Accounts
	if err := json.Unmarshal(data, &a); err != nil {
		return &model.Accounts{}, nil
	}

	return &a, nil
}

func (am *AccountManager) LogoutAccount(email string) error {
	_, err := ExecuteCommand("plumesign", "account", "logout", "-u", email)
	if err != nil {
		log.Err(err).Msgf("Error logout account: %s", email)
		return err
	}
	return nil
}

func (am *AccountManager) GetAccountDevices(email string) ([]model.AccountDevice, error) {
	output, err := ExecuteCommand("plumesign", "account", "devices", "-u", email)
	if err != nil {
		log.Err(err).Msgf("Error getting devices for %s", email)
		return nil, err
	}

	var devices []model.AccountDevice
	content := string(output)
	blocks := strings.Split(content, "Device {")
	for _, block := range blocks {
		if strings.TrimSpace(block) == "" || strings.TrimSpace(block) == "[" || strings.TrimSpace(block) == "]" {
			continue
		}

		var dev model.AccountDevice

		// Helper to extract value
		extract := func(key string) string {
			re := regexp.MustCompile(key + `:\s*"([^"]+)"`)
			matches := re.FindStringSubmatch(block)
			if len(matches) > 1 {
				return matches[1]
			}
			return ""
		}

		dev.DeviceID = extract("device_id")
		dev.Name = extract("name")
		dev.DeviceNumber = extract("device_number")
		dev.DevicePlatform = extract("device_platform")
		dev.Status = extract("status")
		dev.DeviceClass = extract("device_class")

		// Expiration date is special: expiration_date: Some(\n 2026-01-08T14:30:04Z,\n ),
		reDate := regexp.MustCompile(`expiration_date:\s*Some\(\s*([^\s,]+)`)
		matchesDate := reDate.FindStringSubmatch(block)
		if len(matchesDate) > 1 {
			dev.ExpirationDate = matchesDate[1]
		}

		if dev.DeviceID != "" {
			devices = append(devices, dev)
		}
	}

	return devices, nil
}

func (am *AccountManager) DeleteAccountDevice(email, deviceID string) error {
	_, err := ExecuteCommand("plumesign", "account", "delete-device", "-u", email, "--device-id", deviceID)
	if err != nil {
		log.Err(err).Msgf("Error deleting device %s for %s", deviceID, email)
		return err
	}
	return nil
}
