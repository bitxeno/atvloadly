package log

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zerolog.TimeFieldFormat = "2006-01-02 15:04:05"

	consoleWriter := newConsoleWriter(os.Stderr)
	log.Logger = log.With().Caller().Logger().Output(consoleWriter)
}

func AddFileOutput(logPath string) {
	if logPath == "" {
		return
	}

	fmt.Printf("Log file path: %s\n", logPath)
	consoleWriter := newConsoleWriter(os.Stderr)
	// output log to file
	logFileWriter := CreateRollingLogFile(logPath)
	if logFileWriter != nil {
		formatWriter := newFileFormatWriter(logFileWriter)
		multiWriter := zerolog.MultiLevelWriter(consoleWriter, formatWriter)
		log.Logger = log.With().Caller().Logger().Output(multiWriter)
	} else {
		log.Logger = log.With().Caller().Logger().Output(consoleWriter)
	}
}

func SetDebugLevel() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

func SetTraceLevel() {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
}

func Trace(msg string) {
	log.Trace().Msg(msg)
}

func Tracef(format string, v ...interface{}) {
	log.Trace().Msgf(format, v...)
}

func Debug(msg string) {
	log.Debug().Msg(msg)
}

func Debugf(format string, v ...interface{}) {
	log.Debug().Msgf(format, v...)
}

func Print(v ...interface{}) {
	log.Print(v...)
}

func Println(v ...interface{}) {
	log.Print(v...)
}

func Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func Info(msg string) {
	log.Info().Msg(msg)
}

func Infof(format string, v ...interface{}) {
	log.Info().Msgf(format, v...)
}

func Warn(msg string) {
	log.Warn().Msg(msg)
}

func Warnf(format string, v ...interface{}) {
	log.Warn().Msgf(format, v...)
}

func Error(msg string) {
	log.Error().Msg(msg)
}

func Errorf(format string, v ...interface{}) {
	log.Error().Msgf(format, v...)
}

func Err(err error) *zerolog.Event {
	return log.Err(err)
}

func Panic(msg string) {
	log.Panic().Msg(msg)
}

func Panicf(format string, v ...interface{}) {
	log.Panic().Msgf(format, v...)
}

func ColorWarnf(format string, v ...any) {
	color.New(color.FgYellow).Printf(format, v...)
}

func ColorErrorf(format string, v ...any) {
	color.New(color.FgRed).Printf(format, v...)
}

func CreateRollingLogFile(logPath string) io.Writer {
	if logPath == "" {
		return nil
	}

	logDir := path.Dir(logPath)
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		log.Err(err).Msgf("Can't create log directory. path: %s, err: %s", logPath, err)
		return nil
	}

	log.Info().Msgf("Log file: %s", logPath)
	return &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    100, // megabytes
		MaxBackups: 3,
		MaxAge:     7, //daysdays
	}
}
