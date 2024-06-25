package web

import (
	"fmt"
	"math"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
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

	route(server)
	if err := server.Listen(fmt.Sprintf("%s:%d", addr, port)); err != nil {
		log.Error(err.Error())
		return err
	}
	return nil
}
