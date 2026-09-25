package server

import (
	"path/filepath"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/manager"
	"github.com/bitxeno/atvloadly/internal/signing"
	"github.com/bitxeno/atvloadly/internal/task"
	"github.com/bitxeno/atvloadly/web"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
)

var (
	flags = []cli.Flag{
		&cli.IntFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Usage:   "Define an alternate web server port",
		},
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "Load configuration from `FILE`",
		},
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"vv"},
			Usage:   "Enable debug output",
			Value:   false,
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"vvv"},
			Usage:   "Enable verbose output",
			Value:   false,
		},
	}

	Command = &cli.Command{
		Name:   "server",
		Usage:  "Run web server",
		Flags:  flags,
		Action: action,
	}
)

func action(c *cli.Context) error {
	// init config
	debug := c.Bool("debug") || c.Bool("verbose")
	if c.Bool("debug") || c.Bool("verbose") {
		debug = true
	}
	conf, err := app.InitConfig(c.String("config"), debug)
	if err != nil {
		return err
	}
	setttings, err := app.InitSettings(conf, debug)
	if err != nil {
		return err
	}

	// init logger
	if c.Bool("debug") {
		conf.Log.Level = "debug"
		conf.Db.Debug = true
	}
	if c.Bool("verbose") {
		conf.Log.Level = "trace"
		conf.Db.Debug = true
	}
	if err := app.InitLogger(conf); err != nil {
		return err
	}

	// init i18n
	app.InitLanguage(setttings.App.Language)

	// init db
	if err := app.InitDb(conf); err != nil {
		return err
	}
	initSigning(conf)

	// start jobs
	_ = task.ScheduleRefreshApps()
	manager.StartDeviceManager()

	printVersion()
	port := conf.Server.Port
	if c.Int("port") > 0 {
		port = c.Int("port")
	}
	return web.Run(conf.Server.ListenAddr, port)
}

// initSigning configures the external certificate signing mode and removes
// the signing workspaces left behind by a previous crash. Failures are logged
// and do not prevent the server from starting.
func initSigning(conf *app.Configuration) {
	signing.Configure(conf.Signing.KeyFile, filepath.Join(conf.Server.DataDir, "signing-work"))
	removed, err := signing.RecoverWorkspaces()
	if err != nil {
		log.Err(err).Msg("Failed to recover signing workspaces")
		return
	}
	if removed > 0 {
		log.Infof("Removed %d stale signing workspace(s)", removed)
	}
}

func printVersion() {
	_, _ = color.New(color.FgGreen).Print("Starting server version: ")
	_, _ = color.New(color.FgCyan).Printf("%s@%s@%v\n", app.Version.Version, app.Version.BuildDate, app.Mode)
}
