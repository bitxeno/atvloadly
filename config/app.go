package config

import (
	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/creasty/defaults"
)

var App AppConfiguration

type AppConfiguration struct {
	LockdownDir string `koanf:"lockdown_dir" default:"/var/lib/lockdown"`
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
