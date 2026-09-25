package web

import (
	"fmt"
	"math"
	"runtime/debug"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func Run(addr string, port int) error {
	server := fiber.New(fiber.Config{
		BodyLimit: math.MaxInt,
	})

	// set fiber web server access log
	server.Use(logger.New())
	accessWriter := log.CreateRollingLogFile(app.Config.Log.AccessLog)
	if accessWriter != nil {
		server.Use(logger.New(logger.Config{
			Output: accessWriter,
		}))
		log.Infof("Web access log file path: %s", app.Config.Log.AccessLog)
	}

	// A panicking handler answers 500 instead of stopping the server. Recover
	// runs inside the access loggers so they record the 500.
	server.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c *fiber.Ctx, e any) {
			log.Errorf("panic in %s %s: %v\n%s", c.Method(), c.Path(), e, debug.Stack())
		},
	}))

	route(server)
	if err := server.Listen(fmt.Sprintf("%s:%d", addr, port)); err != nil {
		log.Error(err.Error())
		return err
	}
	return nil
}
