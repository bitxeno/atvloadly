package app

import (
	"github.com/bitxeno/atvloadly/internal/cfg"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/mode"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func initLogger() error {
	// set normal log
	log.AddFileOutput(cfg.Server.Log)
	if instance.DebugLog || mode.IsDevelopmentMode() {
		log.SetDebugLevel()
	}
	if instance.VerboseLog {
		log.SetTraceLevel()
	}

	// set fiber web server access log
	instance.server.Use(logger.New())
	accessWriter := log.CreateRollingLogFile(cfg.Server.AccessLog)
	if accessWriter != nil {
		instance.server.Use(logger.New(logger.Config{
			Output: accessWriter,
		}))
		log.Infof("Web access log file path: %s", cfg.Server.AccessLog)
	}
	return nil
}

func initDb() error {
	return nil
}
