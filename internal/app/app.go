package app

import (
	"github.com/bitxeno/atvloadly/internal/cfg"
	"github.com/bitxeno/atvloadly/internal/db"
	"gorm.io/gorm"
)

func Environment() AppMode {
	return Mode
}

// SetAppDataDir points this process at a data directory of its own, instead
// of the platform default ~/.config/<binary name>. Call it before
// InitConfig; the desktop app uses it to keep data in its user directory.
func SetAppDataDir(dir string) {
	cfg.SetAppDataDir(dir)
}

func ReloadConfig() {
}

func Cfg() *Configuration {
	return Config
}

func Db() *gorm.DB {
	return db.Store()
}
