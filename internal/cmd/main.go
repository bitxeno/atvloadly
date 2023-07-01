package cmd

import (
	"os"

	"github.com/bitxeno/atvloadly/internal/version"
	"github.com/urfave/cli/v2"
)

func Run(name string, desc string, arguments []string, webServerAction cli.ActionFunc) (err error) {
	cliApp := &cli.App{
		Name:    name,
		Usage:   desc,
		Version: version.Version,
		Commands: []*cli.Command{
			genCommand,
			{
				Name:   "server",
				Usage:  "Run web server",
				Flags:  serverFlags,
				Action: webServerAction,
			},
		},
	}

	return cliApp.Run(os.Args)
}
