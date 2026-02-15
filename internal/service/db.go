package service

import (
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

func SaveApp(app model.InstalledApp) (*model.InstalledApp, error) {
	// 查找之前的安装记录，存在记录直接更新旧的
	var cur model.InstalledApp
	result := db.Store().Where("udid=? and bundle_identifier=? and account=?", app.UDID, app.BundleIdentifier, app.Account).First(&cur)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		log.Err(result.Error).Msg("SaveApp error.")
		return nil, result.Error
	}

	if result.Error == nil {
		// 之前已安装过
		app.ID = cur.ID

		now := time.Now()
		cur.IpaPath = app.IpaPath
		cur.Icon = app.Icon
		cur.Version = app.Version
		cur.RefreshedDate = &now
		cur.ExpirationDate = app.ExpirationDate
		cur.RefreshedResult = app.RefreshedResult
		cur.RefreshedError = app.RefreshedError
		cur.Password = app.Password

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
		if cur.Icon != "" {
			iconPath := filepath.Join(saveDir, "app.png")
			if err := os.Rename(cur.Icon, iconPath); err != nil {
				log.Err(err).Msgf("Can not move to %s", iconPath)
			} else {
				cur.Icon = iconPath
			}
		}

		updateData := map[string]any{
			"ipa_path":         cur.IpaPath,
			"icon":             cur.Icon,
			"version":          cur.Version,
			"refreshed_date":   cur.RefreshedDate,
			"expiration_date":  cur.ExpirationDate,
			"refreshed_result": cur.RefreshedResult,
			"refreshed_error":  cur.RefreshedError,
			"password":         cur.Password,
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
		if app.Icon != "" {
			iconPath := filepath.Join(saveDir, "app.png")
			if err := os.Rename(app.Icon, iconPath); err != nil {
				log.Err(err).Msgf("Can not move to %s", iconPath)
			} else {
				app.Icon = iconPath
			}
		}
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

func UpdateAppRefreshResult(app model.InstalledApp) error {
	updateData := map[string]any{
		"refreshed_date":   app.RefreshedDate,
		"expiration_date":  app.ExpirationDate,
		"refreshed_result": app.RefreshedResult,
		"refreshed_error":  app.RefreshedError,
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
