package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var instance *sqliteDb

type sqliteDb struct {
	db   *gorm.DB
	path string
	conf Config
}

func new(conf Config) *sqliteDb {
	dbDir := conf.Path
	if _, err := os.Stat(dbDir); err != nil {
		if err := os.MkdirAll(dbDir, os.ModePerm); err != nil {
			log.Panicf("failed to create database directory. path: %s", dbDir)
		}
	}

	return &sqliteDb{
		path: filepath.Join(dbDir, conf.FileName),
		conf: conf,
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
	if s.conf.Debug {
		conf.Logger = logger.Default.LogMode(logger.Info)
	}

	fmt.Printf("Load database from path: %s\n", s.path)
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

func Open(conf Config) *sqliteDb {
	instance = new(conf).Open()

	return instance
}
