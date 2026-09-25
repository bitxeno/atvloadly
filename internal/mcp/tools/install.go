package tools

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/bitxeno/atvloadly/internal/manager"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/service"
	"github.com/bitxeno/atvloadly/internal/signing"
	"github.com/bitxeno/atvloadly/internal/signing/appcheck"
	"github.com/bitxeno/atvloadly/internal/task"
	"github.com/bitxeno/atvloadly/internal/utils"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type installAppInput struct {
	IpaURL            string `json:"ipa_url" jsonschema:"Required IPA download URL"`
	DeviceID          string `json:"device_id,omitempty" jsonschema:"Optional target device ID"`
	AccountID         string `json:"account_id,omitempty" jsonschema:"Optional Apple account ID (md5 of account email). Omit when signing_identity_id is set"`
	SigningIdentityID uint   `json:"signing_identity_id,omitempty" jsonschema:"Optional imported signing identity ID: signs with that external certificate and provisioning profile instead of an Apple account. Omit account_id when set"`
	RemoveExtensions  bool   `json:"remove_extensions,omitempty" jsonschema:"Optional remove app extensions while installing"`
	CustomIdentifier  string `json:"custom_identifier,omitempty" jsonschema:"Optional bundle identifier to install the app under, only with signing_identity_id. The IPA main bundle identifier is replaced by it in the app and its extensions. Omit to keep the IPA bundle identifiers"`
}

type installDeviceOption struct {
	ID             string `json:"id"`
	UDID           string `json:"udid"`
	Name           string `json:"name"`
	DeviceClass    string `json:"device_class"`
	ProductType    string `json:"product_type,omitempty"`
	ProductVersion string `json:"product_version,omitempty"`
}

type installAccountOption struct {
	AccountID    string `json:"account_id"`
	AccountEmail string `json:"account_email"`
	TeamID       string `json:"team_id,omitempty"`
	Status       string `json:"status,omitempty"`
	rawEmail     string
}

// installSigningIdentityOption is an imported signing identity usable by
// install_app with signing_identity_id.
type installSigningIdentityOption struct {
	SigningIdentityID uint      `json:"signing_identity_id"`
	Name              string    `json:"name"`
	TeamID            string    `json:"team_id,omitempty"`
	ProfileName       string    `json:"profile_name,omitempty"`
	ProfileKind       string    `json:"profile_kind,omitempty"`
	ProfilePlatforms  []string  `json:"profile_platforms,omitempty"`
	ExpiresAt         time.Time `json:"expires_at"`
	IsExpired         bool      `json:"is_expired"`
}

type installAppOutput struct {
	Status                     string                         `json:"status"`
	Message                    string                         `json:"message"`
	SelectedDevice             *installDeviceOption           `json:"selected_device,omitempty"`
	SelectedAccount            *installAccountOption          `json:"selected_account,omitempty"`
	SelectedSigningIdentity    *installSigningIdentityOption  `json:"selected_signing_identity,omitempty"`
	AvailableDevices           []installDeviceOption          `json:"available_devices,omitempty"`
	AvailableAccounts          []installAccountOption         `json:"available_accounts,omitempty"`
	AvailableSigningIdentities []installSigningIdentityOption `json:"available_signing_identities,omitempty"`
}

func registerInstallApp(server *sdkmcp.Server) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "install_app",
		Description: "Install/Sideload app to AppleTV with IPA URL. " +
			"The app is signed either with an Apple account (account_id) or with an imported external signing certificate (signing_identity_id), never both. " +
			"If device_id or the signing choice is omitted, this tool returns options for interactive selection. " +
			"Ensure the user has confirmed the device and the account or signing identity before proceeding. " +
			"Apps signed with an external certificate are not refreshed automatically and expire with their signing identity. " +
			"They keep the IPA bundle identifiers unless custom_identifier is set; the provisioning profile decides their application identifier. " +
			"After app install task is queued, call get_install_status to track progress.",
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
	accountID := strings.TrimSpace(input.AccountID)
	if accountID != "" && input.SigningIdentityID != 0 {
		return nil, installAppOutput{}, fmt.Errorf("account_id and signing_identity_id cannot be used together")
	}
	customIdentifier := strings.TrimSpace(input.CustomIdentifier)
	if customIdentifier != "" {
		if input.SigningIdentityID == 0 {
			return nil, installAppOutput{}, fmt.Errorf("custom_identifier requires signing_identity_id")
		}
		// The installation is queued: an invalid identifier is refused now.
		if err := appcheck.ValidateBundleIdentifier(customIdentifier); err != nil {
			return nil, installAppOutput{}, signingToolError(err)
		}
	}

	selectedDevice, deviceOptions, needDeviceChoice, err := resolveDeviceSelection(strings.TrimSpace(input.DeviceID))
	if err != nil {
		return nil, installAppOutput{}, err
	}
	if needDeviceChoice {
		return nil, installAppOutput{
			Status:           "require_device",
			Message:          "Please choose a target device and call install_app again with device_id.",
			AvailableDevices: deviceOptions,
		}, nil
	}

	appModel := model.InstalledApp{
		IpaName:          ipaFileNameFromURL(ipaURL),
		IpaPath:          ipaURL,
		Device:           selectedDevice.Name,
		DeviceClass:      selectedDevice.DeviceClass,
		UDID:             selectedDevice.UDID,
		Enabled:          true,
		RemoveExtensions: input.RemoveExtensions,
	}
	output := installAppOutput{
		Status:         "installing",
		Message:        "Install task queued to task.StartInstallApps.",
		SelectedDevice: selectedDevice,
	}

	if input.SigningIdentityID != 0 {
		identity, err := service.GetSigningIdentity(input.SigningIdentityID)
		if err != nil {
			return nil, installAppOutput{}, signingToolError(err)
		}
		selected := toInstallSigningIdentityOption(*identity, time.Now())
		appModel.SigningMode = model.SigningModeExternalCertificate
		appModel.SigningIdentityID = identity.ID
		appModel.CustomIdentifier = customIdentifier
		output.SelectedSigningIdentity = &selected
	} else {
		if accountID == "" {
			identityOptions, err := listSigningIdentityOptions()
			if err != nil {
				return nil, installAppOutput{}, err
			}
			if len(identityOptions) > 0 {
				// Both signing modes are available: the user chooses.
				accountOptions, err := listAccountOptions()
				if err != nil {
					return nil, installAppOutput{}, err
				}
				return nil, installAppOutput{
					Status:                     "require_signing",
					Message:                    "Please choose an Apple account (account_id) or an external signing identity (signing_identity_id) and call install_app again.",
					SelectedDevice:             selectedDevice,
					AvailableAccounts:          accountOptions,
					AvailableSigningIdentities: identityOptions,
				}, nil
			}
		}

		selectedAccount, accountOptions, needAccountChoice, err := resolveAccountSelection(accountID)
		if err != nil {
			return nil, installAppOutput{}, err
		}
		if needAccountChoice {
			return nil, installAppOutput{
				Status:            "require_account",
				Message:           "Please choose an account and call install_app again with account_id.",
				SelectedDevice:    selectedDevice,
				AvailableAccounts: accountOptions,
			}, nil
		}
		appModel.Account = selectedAccount.rawEmail
		output.SelectedAccount = selectedAccount
	}

	task.StartInstallApps([]model.InstalledApp{appModel}, true)

	return nil, output, nil
}

// signingToolError returns the tool error of err, prefixed with its stable
// signing code when it comes from the external signing pipeline.
func signingToolError(err error) error {
	if code := signing.CodeOf(err); code != "" {
		return fmt.Errorf("%s: %w", code, err)
	}
	return err
}

// listSigningIdentityOptions lists the imported signing identities, newest first.
func listSigningIdentityOptions() ([]installSigningIdentityOption, error) {
	identities, err := service.ListSigningIdentities()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	options := make([]installSigningIdentityOption, 0, len(identities))
	for _, identity := range identities {
		options = append(options, toInstallSigningIdentityOption(identity, now))
	}
	return options, nil
}

func toInstallSigningIdentityOption(identity model.SigningIdentity, now time.Time) installSigningIdentityOption {
	expiresAt := identity.ExpiresAt()
	return installSigningIdentityOption{
		SigningIdentityID: identity.ID,
		Name:              identity.Name,
		TeamID:            identity.TeamID,
		ProfileName:       identity.ProfileName,
		ProfileKind:       string(identity.ProfileKind),
		ProfilePlatforms:  identity.ProfilePlatforms,
		ExpiresAt:         expiresAt,
		IsExpired:         !now.Before(expiresAt),
	}
}

func resolveDeviceSelection(requestedDeviceID string) (*installDeviceOption, []installDeviceOption, bool, error) {
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
			ID:             d.ID,
			UDID:           d.UDID,
			Name:           d.Name,
			DeviceClass:    d.DeviceClass,
			ProductType:    d.ProductType,
			ProductVersion: d.ProductVersion,
		})
	}
	sort.Slice(options, func(i, j int) bool {
		if options[i].Name == options[j].Name {
			return options[i].ID < options[j].ID
		}
		return options[i].Name < options[j].Name
	})

	if requestedDeviceID != "" {
		for _, opt := range options {
			if opt.ID == requestedDeviceID {
				selected := opt
				return &selected, options, false, nil
			}
		}
		return nil, options, false, fmt.Errorf("device_id not found: %s", requestedDeviceID)
	}

	if len(options) == 1 {
		selected := options[0]
		return &selected, options, false, nil
	}

	return nil, options, true, nil
}

func resolveAccountSelection(requestedAccountID string) (*installAccountOption, []installAccountOption, bool, error) {
	options, err := listAccountOptions()
	if err != nil {
		return nil, nil, false, err
	}
	if len(options) == 0 {
		return nil, nil, false, fmt.Errorf("no available apple accounts found")
	}

	if requestedAccountID != "" {
		for _, opt := range options {
			if strings.EqualFold(opt.AccountID, requestedAccountID) {
				selected := opt
				return &selected, options, false, nil
			}
		}
		return nil, options, false, fmt.Errorf("account_id not found: %s", requestedAccountID)
	}

	if len(options) == 1 {
		selected := options[0]
		return &selected, options, false, nil
	}

	return nil, options, true, nil
}

// listAccountOptions lists the logged-in Apple accounts sorted by email.
func listAccountOptions() ([]installAccountOption, error) {
	accounts, err := manager.GetAppleAccounts()
	if err != nil {
		return nil, err
	}
	if accounts == nil {
		return []installAccountOption{}, nil
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
		accountID := utils.Md5(acc.Email)
		options = append(options, installAccountOption{
			AccountID:    accountID,
			AccountEmail: utils.MaskEmail(acc.Email),
			TeamID:       acc.TeamID,
			Status:       acc.Status,
			rawEmail:     acc.Email,
		})
	}
	return options, nil
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

func ipaFileNameFromURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Path == "" {
		return ""
	}

	fileName := path.Base(parsed.Path)
	if fileName == "." || fileName == "/" {
		return ""
	}

	return fileName
}
