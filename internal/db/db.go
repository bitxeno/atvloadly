package db

import (
	"os"
	"path/filepath"

	"github.com/bitxeno/atvloadly/internal/cfg"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/mode"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var instance *sqliteDb

type sqliteDb struct {
	db   *gorm.DB
	path string
}

func New() *sqliteDb {
	dbDir := cfg.Server.WorkDir
	if _, err := os.Stat(dbDir); err != nil {
		if err := os.MkdirAll(dbDir, os.ModePerm); err != nil {
			panic("failed to create database directory")
		}
	}

	return &sqliteDb{
		path: filepath.Join(dbDir, "app.db"),
	}
}

func (s *sqliteDb) SetPath(path string) *sqliteDb {
	s.path = path
	return s
}

func (s *sqliteDb) Open() *sqliteDb {
	if s.db != nil {
		panic("database already open")
	}

	conf := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	if mode.Get() == mode.DevelopmentMode {
		conf.Logger = logger.Default.LogMode(logger.Info)
	}

	db, err := gorm.Open(sqlite.Open(s.path), conf)
	if err != nil {
		panic("failed to open database")
	}

	s.db = db
	return s
}

func (s *sqliteDb) AutoMigrate(dst ...interface{}) error {
	if s.db == nil {
		panic("Database not open: " + s.path)
	}

	if err := s.db.AutoMigrate(dst...); err != nil {
		log.Err(err).Msg("")
		return err
	}

	return nil
}

func Store() *gorm.DB {
	if instance.db == nil {
		panic("Database not open: " + instance.path)
	}

	return instance.db
}

func Open() *sqliteDb {
	instance = New().Open()

	return instance
}
