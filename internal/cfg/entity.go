package cfg

import (
	"path/filepath"

	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/creasty/defaults"
)

var Server = newServerConfiguration()

type ServerConfiguration struct {
	Configuration

	ListenAddr    string `koanf:"listen_addr"`
	Port          int    `koanf:"port" default:"9000"`
	TimeFormat    string `koanf:"time_format" default:"2006-01-02 15:04:05"`
	LogTimeFormat string `koanf:"log_time_format" default:"2006-01-02 15:04:05.000"`
	WorkDir       string `koanf:"work_dir"`
	Log           string `koanf:"log"`
	AccessLog     string `koanf:"access_log"`
}

func newServerConfiguration() *ServerConfiguration {
	return &ServerConfiguration{
		Configuration: *New(),
	}
}

func (c *ServerConfiguration) Load() {
	c.Configuration.Load()

	// set default value
	if err := defaults.Set(c); err != nil {
		log.Panicf("Config set default failed. error: %s \n", err)
	}

	if err := c.ko.Unmarshal("server", c); err != nil {
		log.Panicf("Config unable to decode into struct, %v\n", err)
	}

	// set default data dir
	if c.WorkDir == "" {
		c.WorkDir = defaultConfigDir()
	}
}

func (c *ServerConfiguration) MustLoad() {
	c.Configuration.MustLoad()

	// set default value
	if err := defaults.Set(c); err != nil {
		log.Panicf("Config set default failed. error: %s \n", err)
	}

	if err := c.ko.Unmarshal("server", c); err != nil {
		log.Panicf("Config unable to decode into struct, %v\n", err)
	}

	// set default data dir
	if c.WorkDir == "" {
		c.WorkDir = defaultConfigDir()
	}
}

func (c *ServerConfiguration) Reload() {
	c.Load()
}

func (c *ServerConfiguration) DbPath() string {
	return filepath.Join(c.WorkDir, "app.db")

}
