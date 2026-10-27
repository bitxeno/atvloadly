package app

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/bitxeno/atvloadly/internal/cfg"
	"github.com/bitxeno/atvloadly/internal/db"
	"github.com/bitxeno/atvloadly/internal/i18n"
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
		path = filepath.Join(cfg.AppDataDir(), "config.yaml")
	}
	c, err := cfg.Load(path)
	if err != nil {
		return nil, err
	}
	if err := c.BindStruct(&configuration); err != nil {
		return nil, err
	}
	if configuration.Server.DataDir == "" {
		configuration.Server.DataDir = cfg.AppDataDir()
	}
	if configuration.Signing.KeyFile == "" {
		configuration.Signing.KeyFile = filepath.Join(configuration.Server.DataDir, "keys", "signing-identity.key")
	}
	if configuration.Secret.KeyFile == "" {
		configuration.Secret.KeyFile = filepath.Join(configuration.Server.DataDir, "keys", "secret-store.key")
	}
	// ZEROCONF_ENABLED overrides the config file so containers can toggle
	// device discovery without editing config.yaml. An invalid value is
	// ignored, keeping the configured/default value.
	if v := os.Getenv("ZEROCONF_ENABLED"); v != "" {
		if enabled, err := strconv.ParseBool(v); err == nil {
			configuration.Zeroconf = enabled
		} else {
			log.Warnf("Ignoring invalid ZEROCONF_ENABLED=%q, want true or false", v)
		}
	}
	Config = &configuration

	if debug {
		c.PrintConfig()
	}

	return &configuration, nil
}

// load settings from file
func InitSettings(conf *Configuration, debug bool) (*SettingsConfiguration, error) {
	var settings SettingsConfiguration
	if err := defaults.Set(&settings); err != nil {
		return nil, err
	}

	confDir := conf.Server.DataDir
	if confDir == "" {
		confDir = cfg.AppDataDir()
	}
	path := filepath.Join(confDir, "settings.json")
	c, err := cfg.Load(path)
	if err != nil {
		return nil, err
	}
	if err := c.BindStruct(&settings); err != nil {
		return nil, err
	}
	go startSaveSettingsJob(path)
	Settings = &settings

	if debug {
		// Never dump settings.json verbatim: it holds notification secrets
		// and the sealed GitHub token. Log a redacted summary instead.
		printRedactedSettings()
	}

	return &settings, nil
}

func InitLogger(conf *Configuration) error {
	if conf.Log.LogFile == "" {
		if conf.Server.DataDir != "" {
			conf.Log.LogFile = filepath.Join(conf.Server.DataDir, "app.log")
		} else {
			conf.Log.LogFile = filepath.Join(cfg.AppDataDir(), "app.log")
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
		conf.Db.Path = cfg.AppDataDir()
	}
	if err := db.Open(conf.Db).AutoMigrate(&model.InstalledApp{}, &model.SigningIdentity{}, &model.SavedSource{}); err != nil {
		return err
	}

	return nil
}

func InitLanguage(lang string) {
	i18n.SetLanguage(lang)
}
