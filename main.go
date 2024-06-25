package main

import (
	"fmt"
	"os"

	"github.com/bitxeno/atvloadly/cmd/gen"
	"github.com/bitxeno/atvloadly/cmd/server"
	"github.com/bitxeno/atvloadly/internal/app/build"
	"github.com/go-errors/errors"
	"github.com/urfave/cli/v2"
)

const (
	// service name
	AppName = "atvloadly"
	// service description
	AppDesc = "Publish ipa to AppleTV Easily"
)

func main() {

	cliApp := &cli.App{
		Name:    AppName,
		Usage:   AppDesc,
		Version: build.Version,
		Commands: []*cli.Command{
			gen.Command,
			server.Command,
		},
	}

	if err := cliApp.Run(os.Args); err != nil {
		if e, ok := err.(*errors.Error); ok {
			fmt.Fprintln(os.Stderr, e.ErrorStack())
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
