package app

import (
	"path/filepath"

	"github.com/bitxeno/atvloadly/internal/cfg"
	"github.com/bitxeno/atvloadly/internal/db"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/creasty/defaults"
)

// load config from file
func InitConfig(path string, debug bool) (*Configuration, error) {
	var configuration Configuration
	if err := defaults.Set(&configuration); err != nil {
		return nil, err
	}
	if path == "" {
		path = filepath.Join(cfg.DefaultConfigDir(), "config.yaml")
	}
	c, err := cfg.Load(path)
	if err != nil {
		return nil, err
	}
	if err := c.BindStruct(&configuration); err != nil {
		return nil, err
	}
	if configuration.Server.DataDir == "" {
		configuration.Server.DataDir = cfg.DefaultConfigDir()
	}
	Config = &configuration

	if debug {
		c.PrintConfig()
	}

	return &configuration, nil
}

// load settings from file
func InitSettings(conf *Configuration, debug bool) error {
	var settings SettingsConfiguration
	if err := defaults.Set(&settings); err != nil {
		return err
	}

	confDir := conf.Server.DataDir
	if confDir == "" {
		confDir = cfg.DefaultConfigDir()
	}
	path := filepath.Join(confDir, "settings.json")
	c, err := cfg.Load(path)
	if err != nil {
		return err
	}
	if err := c.BindStruct(&settings); err != nil {
		return err
	}
	go startSaveSettingsJob(path)
	Settings = &settings

	if debug {
		c.PrintConfig()
	}

	return nil
}

func InitLogger(conf *Configuration) error {
	if conf.Log.LogFile == "" {
		if conf.Server.DataDir != "" {
			conf.Log.LogFile = filepath.Join(conf.Server.DataDir, "app.log")
		} else {
			conf.Log.LogFile = filepath.Join(cfg.DefaultConfigDir(), "app.log")
		}
	}
	log.AddFileOutput(conf.Log.LogFile)
	if conf.Log.Level == "debug" {
		log.SetDebugLevel()
	}
	if conf.Log.Level == "trace" {
		log.SetTraceLevel()
	}
	return nil
}

func InitDb(conf *Configuration) error {
	if conf.Db.Path == "" {
		conf.Db.Path = conf.Server.DataDir
	}
	if conf.Db.Path == "" {
		conf.Db.Path = cfg.DefaultConfigDir()
	}
	if err := db.Open(conf.Db).AutoMigrate(&model.InstalledApp{}); err != nil {
		return err
	}

	return nil
}
