//go:build darwin

package cfg

import (
	"log"
	"os"
	"path/filepath"
)

func DefaultConfigDir() string {
	execPath, err := os.Executable()
	if err != nil {
		log.Panic(err)
	}

	execName := filepath.Base(execPath)
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, execName)
}
