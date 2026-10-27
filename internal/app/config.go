package app

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/bitxeno/atvloadly/internal/db"
)

var (
	Config *Configuration
)

// configuration holds any kind of configuration that comes from the outside world and
// is necessary for running the application.
type Configuration struct {
	App struct {
		DeveloperDiskImage struct {
			ImageSource string `koanf:"image_source" json:"image_source" default:"https://raw.githubusercontent.com/bitxeno/DeveloperDiskImages/main"`
			CNProxy     string `koanf:"cn_proxy" json:"cn_proxy" default:"https://mirror.ghproxy.com"`
		} `koanf:"developer_disk_image" json:"developer_disk_image"`
	} `koanf:"app" json:"app"`

	Log struct {
		Level      string `koanf:"level" default:"info"`
		TimeFormat string `koanf:"time_format" default:"2006-01-02 15:04:05.000"`
		LogFile    string `koanf:"log_file"`
		AccessLog  string `koanf:"access_log"`
	} `koanf:"log" json:"log"`

	Server struct {
		ListenAddr string `koanf:"listen_addr" default:"0.0.0.0"`
		Port       int    `koanf:"port" default:"9000"`
		DataDir    string `koanf:"work_dir"`
	} `koanf:"server" json:"server"`

	Db db.Config `koanf:"db" json:"db"`

	Signing struct {
		// KeyFile is the deployment key sealing the private keys of the
		// imported signing identities. Empty means <work_dir>/keys/signing-identity.key.
		KeyFile string `koanf:"key_file" json:"key_file"`
	} `koanf:"signing" json:"signing"`

	Secret struct {
		// KeyFile seals generic secrets such as the GitHub token.
		// Empty means <work_dir>/keys/secret-store.key.
		KeyFile string `koanf:"key_file" json:"key_file"`
	} `koanf:"secret" json:"secret"`

	// Zeroconf enables mDNS device discovery: Avahi on Linux, the built-in
	// zeroconf browser elsewhere. Set it false to run without any discovery —
	// e.g. a manual-IP / Tailscale deployment that adds every device by IP —
	// in which case neither Avahi nor the zeroconf browser starts. It is set
	// from the config file, the ZEROCONF_ENABLED environment variable, or the
	// --zeroconf flag (in that order of precedence), not from the Web UI.
	Zeroconf bool `koanf:"zeroconf" json:"zeroconf" default:"true"`
}

// ZeroconfEnabled reports whether mDNS device discovery is enabled. It is safe
// to call before InitConfig: a nil Config keeps discovery enabled by default.
func ZeroconfEnabled() bool {
	return Config == nil || Config.Zeroconf
}

// SideloadDataDir returns the PlumeImpactor data directory that the plumesign
// engine uses. atvloadly reads the pairing files, accounts and anisette state
// plumesign writes there, so this must resolve to exactly the same directory as
// plumesign's get_data_path: %APPDATA%\PlumeImpactor on Windows, and
// $XDG_CONFIG_HOME/PlumeImpactor (falling back to ~/.config/PlumeImpactor)
// elsewhere. Hard-coding ~/.config broke Windows, where plumesign writes under
// %APPDATA% instead.
func SideloadDataDir() string {
	var base string
	if runtime.GOOS == "windows" {
		base = os.Getenv("APPDATA")
		if base == "" {
			if home, err := os.UserHomeDir(); err == nil {
				base = filepath.Join(home, "AppData", "Roaming")
			}
		}
	} else {
		base = os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			if home, err := os.UserHomeDir(); err == nil {
				base = filepath.Join(home, ".config")
			}
		}
	}
	return filepath.Join(base, "PlumeImpactor")
}

func LockdownDir() string {
	switch runtime.GOOS {
	case "darwin":
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, "/.config/atvloadly/lockdown")
	case "windows":
		if programData := os.Getenv("ProgramData"); programData != "" {
			return filepath.Join(programData, "Apple", "Lockdown")
		}
		return `C:\ProgramData\Apple\Lockdown`
	default:
		return "/var/lib/lockdown"
	}
}

func RemotePairingDir() string {
	return filepath.Join(SideloadDataDir(), "pairing_files")
}
