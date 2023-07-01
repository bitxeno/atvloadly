package service

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bitxeno/atvloadly/internal/cfg"
	"github.com/bitxeno/atvloadly/internal/db"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/model"
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

func SaveApp(app model.InstalledApp) (*model.InstalledApp, error) {
	// 查找之前的安装记录，存在记录直接更新旧的
	var cur model.InstalledApp
	result := db.Store().Where("udid=? and bundle_identifier=? and account=?", app.UDID, app.BundleIdentifier, app.Account).First(&cur)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		log.Err(result.Error).Msg("保存安装记录时出错.")
		return nil, result.Error
	}

	if result.Error == nil {
		// 之前已安装过
		app.ID = cur.ID

		now := time.Now()
		cur.IpaPath = app.IpaPath
		cur.BundleIdentifier = app.BundleIdentifier
		cur.Icon = app.Icon
		cur.Version = app.Version
		cur.RefreshedDate = &now
		cur.RefreshedResult = app.RefreshedResult

		// 把ipa/icon移动到id目录
		saveDir := filepath.Join(cfg.Server.WorkDir, "ipa", fmt.Sprintf("%d", app.ID))
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

		updateData := map[string]interface{}{
			"ipa_path":          cur.IpaPath,
			"bundle_identifier": cur.BundleIdentifier,
			"icon":              cur.Icon,
			"version":           cur.Version,
			"refreshed_date":    cur.RefreshedDate,
			"refreshed_result":  cur.RefreshedResult,
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

		// 把ipa/icon移动到id目录
		saveDir := filepath.Join(cfg.Server.WorkDir, "ipa", fmt.Sprintf("%d", app.ID))
		if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
			panic("failed to create directory :" + saveDir)
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
		updateData := map[string]interface{}{
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
	updateData := map[string]interface{}{
		"refreshed_date":   app.RefreshedDate,
		"refreshed_result": app.RefreshedResult,
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
