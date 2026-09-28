package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	conf "github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/db"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"gorm.io/gorm"
)

func GetApp(id uint) (*model.InstalledApp, error) {
	var apps model.InstalledApp
	if result := db.Store().Where("id = ?", id).First(&apps); result.Error != nil {
		return nil, result.Error
	}

	return &apps, nil
}

func GetAppList() ([]model.InstalledApp, error) {
	var apps []model.InstalledApp
	if result := db.Store().Order("created_at desc").Find(&apps); result.Error != nil {
		return nil, result.Error
	}

	return apps, nil
}

func GetEnableAppList() ([]model.InstalledApp, error) {
	var apps []model.InstalledApp
	if result := db.Store().Where("enabled = ?", true).Order("created_at desc").Find(&apps); result.Error != nil {
		return nil, result.Error
	}

	return apps, nil
}

func GetEnableAppListByUDID(udid string) ([]model.InstalledApp, error) {
	var apps []model.InstalledApp
	if result := db.Store().Where("enabled = ? AND udid = ?", true, udid).Order("created_at desc").Find(&apps); result.Error != nil {
		return nil, result.Error
	}

	return apps, nil
}

// HasExpiredApps checks if there are any enabled apps that have expired
func HasExpiredApps() (bool, error) {
	var apps []model.InstalledApp
	if result := db.Store().Where("enabled = ?", true).Find(&apps); result.Error != nil {
		return false, result.Error
	}

	for _, app := range apps {
		if app.IsExpired() {
			return true, nil
		}
	}

	return false, nil
}

// SaveApp records a successful installation, updating the previous record of
// the same app when there is one (see findInstalledApp). The IPA and the icon
// are moved into the directory of the record; the client provided icon only
// when it is a regular file of the upload or installed apps directories (see
// storeAppIcon).
func SaveApp(app model.InstalledApp) (*model.InstalledApp, error) {
	app.SigningMode = app.EffectiveSigningMode()
	if !app.SigningMode.IsValid() {
		return nil, fmt.Errorf("invalid signing mode: %q", app.SigningMode)
	}

	// 查找之前的安装记录，存在记录直接更新旧的
	cur, found, err := findInstalledApp(app)
	if err != nil {
		log.Err(err).Msg("SaveApp error.")
		return nil, err
	}

	if found {
		// 之前已安装过
		app.ID = cur.ID

		now := time.Now()
		cur.IpaName = app.IpaName
		cur.IpaPath = app.IpaPath
		cur.Version = app.Version
		cur.RefreshedDate = &now
		cur.ExpirationDate = app.ExpirationDate
		cur.RefreshedResult = app.RefreshedResult
		cur.RefreshedError = app.RefreshedError
		cur.Password = app.Password
		cur.CustomName = app.CustomName
		cur.SigningMode = app.SigningMode
		cur.SigningIdentityID = app.SigningIdentityID
		cur.SignedBundleIdentifier = app.SignedBundleIdentifier
		cur.CustomIdentifier = app.CustomIdentifier
		cur.AllowMissingEntitlements = app.AllowMissingEntitlements
		// The source link describes how the app was last installed: installing
		// from a file or URL stops tracking.
		cur.Source = app.Source

		// 把 ipa/icon 移动到 ipa 保存目录
		saveDir := filepath.Join(conf.Config.Server.DataDir, "ipa", fmt.Sprintf("%d", app.ID))
		if cur.IpaPath != "" {
			ipaPath := filepath.Join(saveDir, "app.ipa")
			if err := os.Rename(cur.IpaPath, ipaPath); err != nil {
				log.Err(err).Msgf("Can not move to %s", ipaPath)
			} else {
				cur.IpaPath = ipaPath
			}
		}
		cur.Icon = storeAppIcon(app.Icon, saveDir, cur.Icon)

		updateData := map[string]any{
			"ipa_name":                   cur.IpaName,
			"ipa_path":                   cur.IpaPath,
			"icon":                       cur.Icon,
			"version":                    cur.Version,
			"refreshed_date":             cur.RefreshedDate,
			"expiration_date":            cur.ExpirationDate,
			"refreshed_result":           cur.RefreshedResult,
			"refreshed_error":            cur.RefreshedError,
			"password":                   cur.Password,
			"custom_name":                cur.CustomName,
			"signing_mode":               cur.SigningMode,
			"signing_identity_id":        cur.SigningIdentityID,
			"signed_bundle_identifier":   cur.SignedBundleIdentifier,
			"custom_identifier":          cur.CustomIdentifier,
			"allow_missing_entitlements": cur.AllowMissingEntitlements,
		}
		if cur.IsExternalSigning() {
			// The next reinstall signs with the same plan as this installation.
			cur.RemoveExtensions = app.RemoveExtensions
			updateData["remove_extensions"] = cur.RemoveExtensions
			// The record is keyed by the app on the device, which another IPA
			// can replace when it is installed under the same identifier.
			cur.BundleIdentifier = app.BundleIdentifier
			updateData["bundle_identifier"] = cur.BundleIdentifier
		}
		for k, v := range sourceColumns(cur.Source) {
			updateData[k] = v
		}
		if result := db.Store().Model(&cur).Updates(updateData); result.Error != nil {
			return nil, result.Error
		}

		return &cur, nil
	} else {
		// 新安装
		now := time.Now()
		app.Enabled = true
		app.InstalledDate = &now
		// The client provided icon is recorded only once moved.
		icon := app.Icon
		app.Icon = ""
		if result := db.Store().Create(&app); result.Error != nil {
			return nil, result.Error
		}

		// 把 ipa/icon 移动到 ipa 保存目录
		saveDir := filepath.Join(conf.Config.Server.DataDir, "ipa", fmt.Sprintf("%d", app.ID))
		if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("failed to create directory : %s, error: %s", saveDir, err)
		}
		if app.IpaPath != "" {
			ipaPath := filepath.Join(saveDir, "app.ipa")
			if err := os.Rename(app.IpaPath, ipaPath); err != nil {
				log.Err(err).Msgf("Can not move to %s", ipaPath)
			} else {
				app.IpaPath = ipaPath
			}
		}
		app.Icon = storeAppIcon(icon, saveDir, "")
		updateData := map[string]any{
			"ipa_path": app.IpaPath,
			"icon":     app.Icon,
		}
		if result := db.Store().Model(&app).Updates(updateData); result.Error != nil {
			return nil, result.Error
		}

		return &app, nil
	}
}

// storeAppIcon moves the icon of an installation into saveDir and returns the
// icon path to record, or current when the icon was not moved. The icon path
// comes from the client: only a regular file inside the upload directory
// (<DataDir>/tmp, where the IPA parsers extract icons) or the installed apps
// directory (<DataDir>/ipa) is moved, symbolic links followed.
func storeAppIcon(icon, saveDir, current string) string {
	if icon == "" {
		return current
	}
	dataDir := conf.Config.Server.DataDir
	resolved, err := resolvePathWithin(icon, filepath.Join(dataDir, "tmp"), filepath.Join(dataDir, "ipa"))
	if err != nil {
		log.Warnf("Ignoring the app icon %s: %s", icon, err)
		return current
	}
	iconPath := filepath.Join(saveDir, "app.png")
	if err := os.Rename(resolved, iconPath); err != nil {
		log.Err(err).Msgf("Can not move to %s", iconPath)
		return current
	}
	return iconPath
}

// ResolveAppIconPath returns the symlink-free path of the recorded icon of an
// installed app when it is a regular file inside the installed apps directory
// (<DataDir>/ipa); anything else is an error.
func ResolveAppIconPath(icon string) (string, error) {
	return resolvePathWithin(icon, filepath.Join(conf.Config.Server.DataDir, "ipa"))
}

// findInstalledApp returns the previous record of app for SaveApp. Records of
// different signing modes are never merged. An Apple ID record is keyed by
// device, bundle identifier and account. An external certificate record is
// keyed by the app actually on the device: device, main bundle identifier
// installed (see signedBundleIdentifierOf) and signing identity, so the same
// IPA installed under another custom identifier gets its own record.
func findInstalledApp(app model.InstalledApp) (model.InstalledApp, bool, error) {
	var cur model.InstalledApp
	query := db.Store().Where("udid = ?", app.UDID)
	if app.IsExternalSigning() {
		query = query.Where("signed_bundle_identifier = ? AND signing_mode = ? AND signing_identity_id = ?", signedBundleIdentifierOf(app), model.SigningModeExternalCertificate, app.SigningIdentityID)
	} else {
		query = query.Where("bundle_identifier = ? AND account = ? AND (signing_mode IS NULL OR signing_mode IN ?)", app.BundleIdentifier, app.Account, []model.SigningMode{"", model.SigningModeAppleID})
	}
	result := query.First(&cur)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return cur, false, nil
	}
	if result.Error != nil {
		return cur, false, result.Error
	}
	return cur, true, nil
}

// signedBundleIdentifierOf returns the main bundle identifier an external
// certificate installation of app puts on the device: the one recorded from
// its plan, else the custom identifier (the engine installs the main app
// under it), else the bundle identifier of the IPA.
func signedBundleIdentifierOf(app model.InstalledApp) string {
	switch {
	case app.SignedBundleIdentifier != "":
		return app.SignedBundleIdentifier
	case app.CustomIdentifier != "":
		return app.CustomIdentifier
	default:
		return app.BundleIdentifier
	}
}

// UpdateAppRefreshResult records the outcome of a refresh or reinstallation.
func UpdateAppRefreshResult(app model.InstalledApp) error {
	updateData := map[string]any{
		"refreshed_date":   app.RefreshedDate,
		"expiration_date":  app.ExpirationDate,
		"refreshed_result": app.RefreshedResult,
		"refreshed_error":  app.RefreshedError,
	}
	if app.IsExternalSigning() && app.RefreshedResult {
		// The record keeps the main bundle identifier actually installed on the device.
		updateData["signed_bundle_identifier"] = app.SignedBundleIdentifier
	}
	if result := db.Store().Model(&app).Updates(updateData); result.Error != nil {
		return result.Error
	}

	return nil
}

func DeleteApp(id uint) (bool, error) {
	if v, err := GetApp(id); err == nil {
		if result := db.Store().Delete(&model.InstalledApp{}, id); result.Error != nil {
			return false, result.Error
		}
		ipaDir := filepath.Dir(v.IpaPath)
		_ = os.RemoveAll(ipaDir)
	}

	return true, nil
}
