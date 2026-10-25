package server

import (
	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/bootstrap"
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
	printVersion()
	_, stop, err := bootstrap.Run(bootstrap.Options{
		ConfigFile: c.String("config"),
		Debug:      c.Bool("debug"),
		Trace:      c.Bool("verbose"),
		Port:       c.Int("port"),
	})
	if err != nil {
		return err
	}
	defer stop()

	// The web server serves until the process is interrupted.
	select {}
}

func printVersion() {
	_, _ = color.New(color.FgGreen).Print("Starting server version: ")
	_, _ = color.New(color.FgCyan).Printf("%s@%s@%v\n", app.Version.Version, app.Version.BuildDate, app.Mode)
}
