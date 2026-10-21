//go:build darwin

package cfg

import (
	"os"
	"path/filepath"
)

func DefaultConfigDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(dir, "atvloadly")
}
