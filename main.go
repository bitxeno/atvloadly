package main

import (
	"fmt"
	"os"

	"github.com/bitxeno/atvloadly/config"
	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/db"
	_ "github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/manager"
	"github.com/bitxeno/atvloadly/model"
	"github.com/bitxeno/atvloadly/router"
	"github.com/bitxeno/atvloadly/task"
	"github.com/gofiber/fiber/v2"
)

const (
	// service name
	AppName = "atvloadly"
	// service description
	AppDesc = "Publish ipa to AppleTV Easily"
)

func main() {
	app := app.New(AppName, AppDesc)
	app.Route(func(f *fiber.App) {
		router.Create(f, getViewAssets())
	})
	app.AddBoot(func() error {
		if err := config.Load(); err != nil {
			return err
		}
		if err := db.Open().AutoMigrate(&model.InstalledApp{}); err != nil {
			return err
		}
		_ = task.ScheduleRefreshApps()
		manager.StartDeviceManager()
		return nil
	})

	if err := app.Run(os.Args); err != nil {
		code := 1
		fmt.Fprintln(os.Stderr, err)
		os.Exit(code)
	}
}
