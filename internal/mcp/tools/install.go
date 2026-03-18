package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bitxeno/atvloadly/internal/app"
	atvhttp "github.com/bitxeno/atvloadly/internal/http"
	"github.com/bitxeno/atvloadly/internal/ipa"
	"github.com/bitxeno/atvloadly/internal/manager"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/service"
	"github.com/bitxeno/atvloadly/internal/task"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type installAppInput struct {
	IpaURL           string `json:"ipa_url" jsonschema:"Required IPA download URL"`
	DeviceUDID       string `json:"device_udid,omitempty" jsonschema:"Optional target device UDID"`
	AccountEmail     string `json:"account_email,omitempty" jsonschema:"Optional Apple account email"`
	RemoveExtensions bool   `json:"remove_extensions,omitempty" jsonschema:"Optional remove app extensions while installing"`
}

type installDeviceOption struct {
	UDID           string `json:"udid"`
	Name           string `json:"name"`
	DeviceClass    string `json:"device_class"`
	ProductType    string `json:"product_type,omitempty"`
	ProductVersion string `json:"product_version,omitempty"`
}

type installAccountOption struct {
	Email  string `json:"email"`
	TeamID string `json:"team_id,omitempty"`
	Status string `json:"status,omitempty"`
}

type installAppOutput struct {
	Status            string                 `json:"status"`
	Message           string                 `json:"message"`
	AppID             uint                   `json:"app_id,omitempty"`
	SelectedDevice    *installDeviceOption   `json:"selected_device,omitempty"`
	SelectedAccount   *installAccountOption  `json:"selected_account,omitempty"`
	AvailableDevices  []installDeviceOption  `json:"available_devices,omitempty"`
	AvailableAccounts []installAccountOption `json:"available_accounts,omitempty"`
}

func registerInstallApp(server *sdkmcp.Server) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "install_app",
		Description: "Install app from IPA URL. " +
			"If device_udid/account_email is omitted, this tool returns options for interactive selection. " +
			"When there is only one available device/account, it is selected automatically. " +
			"After app is queued, use get_install_status with app_id to track progress.",
	}, handleInstallApp)
}

func handleInstallApp(_ context.Context, _ *sdkmcp.CallToolRequest, input installAppInput) (*sdkmcp.CallToolResult, installAppOutput, error) {
	ipaURL := strings.TrimSpace(input.IpaURL)
	if ipaURL == "" {
		return nil, installAppOutput{}, fmt.Errorf("ipa_url is required")
	}
	if !isIPAURL(ipaURL) {
		return nil, installAppOutput{}, fmt.Errorf("ipa_url must point to an .ipa file")
	}

	selectedDevice, deviceOptions, needDeviceChoice, err := resolveDeviceSelection(strings.TrimSpace(input.DeviceUDID))
	if err != nil {
		return nil, installAppOutput{}, err
	}
	if needDeviceChoice {
		return nil, installAppOutput{
			Status:           "require_device",
			Message:          "Please choose a target device and call install_app again with device_udid.",
			AvailableDevices: deviceOptions,
		}, nil
	}

	selectedAccount, accountOptions, needAccountChoice, err := resolveAccountSelection(strings.TrimSpace(input.AccountEmail))
	if err != nil {
		return nil, installAppOutput{}, err
	}
	if needAccountChoice {
		return nil, installAppOutput{
			Status:            "require_account",
			Message:           "Please choose an account and call install_app again with account_email.",
			SelectedDevice:    selectedDevice,
			AvailableAccounts: accountOptions,
		}, nil
	}

	tmpDir := filepath.Join(app.Config.Server.DataDir, "tmp")
	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
		return nil, installAppOutput{}, fmt.Errorf("failed to create temporary directory: %w", err)
	}

	tmpFile, err := os.CreateTemp(tmpDir, "mcp_*.ipa")
	if err != nil {
		return nil, installAppOutput{}, fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpIPAPath := tmpFile.Name()
	_ = tmpFile.Close()

	resp, err := atvhttp.NewClient().R().SetOutput(tmpIPAPath).Get(ipaURL)
	if err != nil {
		_ = os.Remove(tmpIPAPath)
		return nil, installAppOutput{}, fmt.Errorf("failed to download ipa: %w", err)
	}
	if !resp.IsSuccess() {
		_ = os.Remove(tmpIPAPath)
		return nil, installAppOutput{}, fmt.Errorf("download failed with status code %d", resp.StatusCode())
	}

	packageInfo, err := ipa.ParseFile(tmpIPAPath)
	if err != nil {
		_ = os.Remove(tmpIPAPath)
		return nil, installAppOutput{}, fmt.Errorf("downloaded file is not a valid ipa: %w", err)
	}

	appModel := model.InstalledApp{
		IpaName:          packageInfo.Name(),
		IpaPath:          tmpIPAPath,
		Device:           selectedDevice.Name,
		DeviceClass:      selectedDevice.DeviceClass,
		UDID:             selectedDevice.UDID,
		Account:          selectedAccount.Email,
		BundleIdentifier: packageInfo.Identifier(),
		Version:          packageInfo.Version(),
		Enabled:          true,
		RemoveExtensions: input.RemoveExtensions,
	}

	savedApp, err := service.SaveApp(appModel)
	if err != nil {
		_ = os.Remove(tmpIPAPath)
		return nil, installAppOutput{}, fmt.Errorf("save app record failed: %w", err)
	}

	task.StartInstallApps([]model.InstalledApp{*savedApp}, true)

	return nil, installAppOutput{
		Status:          "installing",
		Message:         "Install task queued to task.StartInstallApps.",
		AppID:           savedApp.ID,
		SelectedDevice:  selectedDevice,
		SelectedAccount: selectedAccount,
	}, nil
}

func resolveDeviceSelection(requestedUDID string) (*installDeviceOption, []installDeviceOption, bool, error) {
	manager.ReloadDevices()
	devices, err := manager.GetDevices()
	if err != nil {
		return nil, nil, false, err
	}
	if len(devices) == 0 {
		return nil, nil, false, fmt.Errorf("no available devices found")
	}

	options := make([]installDeviceOption, 0, len(devices))
	for _, d := range devices {
		options = append(options, installDeviceOption{
			UDID:           d.UDID,
			Name:           d.Name,
			DeviceClass:    d.DeviceClass,
			ProductType:    d.ProductType,
			ProductVersion: d.ProductVersion,
		})
	}
	sort.Slice(options, func(i, j int) bool {
		if options[i].Name == options[j].Name {
			return options[i].UDID < options[j].UDID
		}
		return options[i].Name < options[j].Name
	})

	if requestedUDID != "" {
		for _, opt := range options {
			if opt.UDID == requestedUDID {
				selected := opt
				return &selected, options, false, nil
			}
		}
		return nil, options, false, fmt.Errorf("device_udid not found: %s", requestedUDID)
	}

	if len(options) == 1 {
		selected := options[0]
		return &selected, options, false, nil
	}

	return nil, options, true, nil
}

func resolveAccountSelection(requestedEmail string) (*installAccountOption, []installAccountOption, bool, error) {
	accounts, err := manager.GetAppleAccounts()
	if err != nil {
		return nil, nil, false, err
	}
	if accounts == nil || len(accounts.Accounts) == 0 {
		return nil, nil, false, fmt.Errorf("no available apple accounts found")
	}

	emails := make([]string, 0, len(accounts.Accounts))
	for email := range accounts.Accounts {
		emails = append(emails, email)
	}
	sort.Strings(emails)

	options := make([]installAccountOption, 0, len(emails))
	for _, email := range emails {
		acc := accounts.Accounts[email]
		if acc.Email == "" {
			acc.Email = email
		}
		options = append(options, installAccountOption{
			Email:  acc.Email,
			TeamID: acc.TeamID,
			Status: acc.Status,
		})
	}

	if requestedEmail != "" {
		for _, opt := range options {
			if strings.EqualFold(opt.Email, requestedEmail) {
				selected := opt
				return &selected, options, false, nil
			}
		}
		return nil, options, false, fmt.Errorf("account_email not found: %s", requestedEmail)
	}

	if len(options) == 1 {
		selected := options[0]
		return &selected, options, false, nil
	}

	return nil, options, true, nil
}

func isIPAURL(rawURL string) bool {
	cleaned := strings.ToLower(strings.TrimSpace(rawURL))
	if cleaned == "" {
		return false
	}
	if idx := strings.Index(cleaned, "?"); idx >= 0 {
		cleaned = cleaned[:idx]
	}
	return strings.HasSuffix(cleaned, ".ipa")
}
