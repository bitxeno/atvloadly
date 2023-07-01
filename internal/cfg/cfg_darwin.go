package cfg

import (
	"os"
	"path/filepath"
)

func defaultConfigDir() string {
	execPath, err := os.Executable()
	if err != nil {
		panic(err)
	}

	execName := filepath.Base(execPath)
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, execName)
}
