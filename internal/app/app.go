package app

import (
	"fmt"
	"math"

	"github.com/bitxeno/atvloadly/internal/cfg"
	"github.com/bitxeno/atvloadly/internal/cmd"
	"github.com/bitxeno/atvloadly/internal/db"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/mode"
	"github.com/bitxeno/atvloadly/internal/version"
	"github.com/fatih/color"
	"github.com/gofiber/fiber/v2"
	"github.com/urfave/cli/v2"
	"gorm.io/gorm"
)

var instance *Application

type Application struct {
	Name       string
	Desc       string
	Version    version.Info
	Mode       mode.AppMode // 用于加载不同环境变量和配置文件
	Port       int
	DebugLog   bool
	VerboseLog bool

	server  *fiber.App
	routers []RouteFunc
}

func New(name string, desc string) *Application {
	instance = &Application{
		Name:       name,
		Desc:       desc,
		Version:    version.Get(),
		Mode:       mode.Get(),
		DebugLog:   false,
		VerboseLog: false,
		server: fiber.New(fiber.Config{
			BodyLimit: math.MaxInt,
		}),
		routers: []RouteFunc{},
	}

	return instance
}

func (a *Application) AddBoot(fn BootFunc) {
	addBoot(fn)
}

func (a *Application) AddBoots(boots ...Bootstrapper) {
	addBoots(boots...)
}

func (a *Application) Route(fn RouteFunc) {
	a.routers = append(a.routers, fn)
}

func (a *Application) Run(arguments []string) (err error) {
	return cmd.Run(a.Name, a.Desc, arguments, func(c *cli.Context) error {
		a.Port = c.Int("port")
		a.DebugLog = c.Bool("debug")
		a.VerboseLog = c.Bool("verbose")

		a.runWeb(c.String("config"))
		return nil
	})
}

func (a *Application) runWeb(configFile string) {
	// load config. 配置优先级：命令行参数 > 环境变量 > 配置文件
	if configFile != "" {
		cfg.Server.SetPath(configFile)
	}
	cfg.Server.Load()

	// run bootstrap middleware
	bootLauncher.Prepend(
		BootFunc(initLogger),
		BootFunc(initDb),
	)
	bootLauncher.Run()

	// add web router
	for _, router := range a.routers {
		router(a.server)
	}

	// run web server
	listenPort := a.Port
	if listenPort <= 0 {
		listenPort = cfg.Server.Port
	}

	a.printAppVersion()
	err := a.server.Listen(fmt.Sprintf("%s:%d", cfg.Server.ListenAddr, listenPort))
	if err != nil {
		log.Error(err.Error())
	}
}

func (a *Application) printAppVersion() {
	color.New(color.FgGreen).Printf("Starting %s version: ", a.Name)
	color.New(color.FgCyan).Printf("%s@%s@%v\n", a.Version.Version, a.Version.BuildDate, a.Mode)
}

func (a *Application) SetConfigPath(path string) {
	cfg.Server.SetPath(path)
}

func Environment() mode.AppMode {
	return instance.Mode
}

func DevelopmentMode() bool {
	return mode.IsDevelopmentMode()
}

func Name() string {
	return instance.Name
}

func Version() version.Info {
	return instance.Version
}

func ReloadConfig() {
	cfg.Server.Reload()
}

func Cfg() *cfg.ServerConfiguration {
	return cfg.Server
}

func Db() *gorm.DB {
	return db.Store()
}
