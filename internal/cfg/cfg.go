package cfg

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/bitxeno/atvloadly/internal/utils"
	"github.com/creasty/defaults"
	"github.com/go-errors/errors"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
)

type configuration struct {
	ko   *koanf.Koanf
	path string
}

func New() *configuration {
	return &configuration{
		ko:   koanf.New("."),
		path: filepath.Join(DefaultConfigDir(), "config.yaml"),
	}
}

func (c *configuration) load(path string) error {
	if path != "" {
		c.path = path
	}

	// set default value
	if err := defaults.Set(c); err != nil {
		return errors.New(err)
	}

	// read config from file
	if !utils.Exists(c.path) {
		fmt.Printf("[WARN] Config file not exists. path: %s\n", c.path)
		return nil
	}

	fmt.Printf("Load config from path: %s\n", c.path)
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

	return nil
}

func (c *configuration) BindStruct(dst any) error {
	return c.ko.Unmarshal("", dst)
}

func (c *configuration) Reload() {
	_ = c.load("")
}

func (c *configuration) PrintConfig() {
	configName := filepath.Base(c.path)
	fmt.Printf("##################### Load %s begin #####################\n", configName)
	c.ko.Print()
	fmt.Printf("#####################  Load %s end  #####################\n", configName)
}

func Load(path string) (*configuration, error) {
	conf := New()
	if err := conf.load(path); err != nil {
		return nil, err
	}

	return conf, nil
}

// appDataDir overrides DefaultConfigDir when set; the desktop app points it
// at its user data directory.
var appDataDir string

// SetAppDataDir overrides the default data directory for this process.
func SetAppDataDir(dir string) {
	appDataDir = dir
}

// AppDataDir returns the directory this process keeps its data in: the
// override set by SetAppDataDir, or the platform default.
func AppDataDir() string {
	if appDataDir != "" {
		return appDataDir
	}
	return DefaultConfigDir()
}
