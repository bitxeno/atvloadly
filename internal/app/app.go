package app

import (
	"github.com/bitxeno/atvloadly/internal/db"
	"gorm.io/gorm"
)

func Environment() AppMode {
	return Mode
}

func ReloadConfig() {
}

func Cfg() *Configuration {
	return Config
}

func Db() *gorm.DB {
	return db.Store()
}
