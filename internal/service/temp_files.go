package service

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	conf "github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/log"
)

// staleTempFileAge is the age from which a file staged in the upload directory
// is considered abandoned.
const staleTempFileAge = 24 * time.Hour

// RemoveStaleTempFiles removes the regular files directly under <DataDir>/tmp
// last modified more than 24 hours ago and returns how many were removed. The
// install page stages uploads, source downloads and their icons there and
// releases them itself; a closed tab or a client timeout leaves them behind.
// Directories and symbolic links are kept, a missing directory is nothing to
// do. Files that cannot be removed are reported in the error, after the others
// were removed.
func RemoveStaleTempFiles() (int, error) {
	dir := filepath.Join(conf.Config.Server.DataDir, "tmp")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to list the staged files in %s: %w", dir, err)
	}

	cutoff := time.Now().Add(-staleTempFileAge)
	removed := 0
	var errs []error
	for _, entry := range entries {
		// The entry type describes a symbolic link itself, never its target.
		if !entry.Type().IsRegular() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to stat the staged file %s: %w", path, err))
			continue
		}
		if !info.ModTime().Before(cutoff) {
			continue
		}
		if err := os.Remove(path); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, fmt.Errorf("failed to remove the stale staged file %s: %w", path, err))
			}
			continue
		}
		log.Infof("Removed the stale staged file %s", path)
		removed++
	}
	return removed, errors.Join(errs...)
}
