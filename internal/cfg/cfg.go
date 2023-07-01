package cfg

import (
	"fmt"
	"path/filepath"

	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/utils"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
)

type Configuration struct {
	ko   *koanf.Koanf
	path string
}

func New() *Configuration {
	return &Configuration{
		ko:   koanf.New("."),
		path: filepath.Join(defaultConfigDir(), "config.yaml"),
	}
}

func (c *Configuration) SetPath(path string) {
	if filepath.IsAbs(path) {
		c.path = path
	} else {
		c.path = filepath.Join(defaultConfigDir(), path)
	}
}

func (c *Configuration) Path() string {
	return c.path
}

func (c *Configuration) Load() {
	// read config
	if utils.Exists(c.path) {
		ext := filepath.Ext(c.path)
		if ext == ".yaml" || ext == ".yml" {
			if err := c.ko.Load(file.Provider(c.path), yaml.Parser()); err != nil {
				log.Panicf("Yaml config file read failed. error: %s \n", err)
			}
		}

		if ext == ".json" {
			if err := c.ko.Load(file.Provider(c.path), json.Parser()); err != nil {
				log.Panicf("Json config file read failed. error: %s \n", err)
			}
		}
	} else {
		fmt.Printf("Config file not exists. file: %s\n", c.path)
	}

	c.printConfig()
}

func (c *Configuration) MustLoad() {
	if !utils.Exists(c.path) {
		log.Panicf("Config file not exists. file: %s \n", c.path)
	}

	// read config
	ext := filepath.Ext(c.path)
	if ext == ".yaml" || ext == ".yml" {
		if err := c.ko.Load(file.Provider(c.path), yaml.Parser()); err != nil {
			log.Panicf("Config file read failed. error: %s \n", err)
		}
	}

	if ext == ".json" {
		if err := c.ko.Load(file.Provider(c.path), json.Parser()); err != nil {
			log.Panicf("Config file read failed. error: %s \n", err)
		}
	}

	c.printConfig()
}

func (c *Configuration) BindStruct(key string, dst any) error {
	return c.ko.Unmarshal(key, dst)
}

func (c *Configuration) Reload() {
	c.Load()
}
