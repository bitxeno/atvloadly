package app

import (
	"fmt"

	"github.com/bitxeno/atvloadly/internal/db"
)

var (
	Config *Configuration
)

// configuration holds any kind of configuration that comes from the outside world and
// is necessary for running the application.
type Configuration struct {
	App struct {
		LockdownDir        string `koanf:"lockdown_dir" default:"/var/lib/lockdown"`
		DeveloperDiskImage struct {
			ImageSource string `koanf:"image_source" json:"image_source" default:"https://github.com/haikieu/xcode-developer-disk-image-all-platforms/raw/master/DiskImages/AppleTVOS.platform/DeviceSupport/{0}.zip"`
			CNProxy     string `koanf:"cn_proxy" json:"cn_proxy" default:"https://mirror.ghproxy.com"`
		} `koanf:"developer_disk_image" json:"developer_disk_image"`
	}

	Log struct {
		Level      string `koanf:"level" default:"info"`
		TimeFormat string `koanf:"time_format" default:"2006-01-02 15:04:05.000"`
		LogFile    string `koanf:"log_file"`
		AccessLog  string `koanf:"access_log"`
	} `koanf:"log" json:"log"`

	Server struct {
		ListenAddr string `koanf:"listen_addr" default:"0.0.0.0"`
		Port       int    `koanf:"port" default:"9000"`
		DataDir    string `koanf:"data_dir"`
	} `koanf:"app" json:"app"`

	Db db.Config `koanf:"db" json:"db"`
}

func SideloaderDataDir() string {
	return fmt.Sprintf("%s/Sideloader", Config.Server.DataDir)
}
