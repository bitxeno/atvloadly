package config

import (
	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/creasty/defaults"
)

var App AppConfiguration

type AppConfiguration struct {
	LockdownDir        string `koanf:"lockdown_dir" default:"/var/lib/lockdown"`
	DeveloperDiskImage struct {
		ImageSource string `koanf:"image_source" json:"image_source" default:"https://ghproxy.com/https://github.com/haikieu/xcode-developer-disk-image-all-platforms/raw/master/DiskImages/AppleTVOS.platform/DeviceSupport/{0}.zip"`
	} `koanf:"developer_disk_image" json:"developer_disk_image"`
}

func loadApp() error {
	// set default value
	if err := defaults.Set(&App); err != nil {
		return err
	}
	if err := app.Cfg().BindStruct("app", &App); err != nil {
		return err
	}

	return nil
}
