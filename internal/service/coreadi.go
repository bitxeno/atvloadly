package service

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

var coreADIUpdateMu sync.Mutex

// installCoreADILibraries extracts both required libraries before replacing
// either installed file. An incomplete or corrupt APK leaves existing libs intact.
func installCoreADILibraries(apkPath, destDir, pkgArch string) error {
	archive, err := zip.OpenReader(apkPath)
	if err != nil {
		return fmt.Errorf("open apk zip failed: %w", err)
	}
	defer func() { _ = archive.Close() }()

	parent := filepath.Dir(destDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create lib parent dir failed: %w", err)
	}
	stagingDir, err := os.MkdirTemp(parent, ".coreadi-*")
	if err != nil {
		return fmt.Errorf("create lib staging dir failed: %w", err)
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()

	names := []string{"libstoreservicescore.so", "libCoreADI.so"}
	required := make(map[string]string, len(names))
	for _, name := range names {
		required[filepath.ToSlash(filepath.Join("lib", pkgArch, name))] = name
	}
	for _, file := range archive.File {
		name, wanted := required[file.Name]
		if !wanted {
			continue
		}
		src, err := file.Open()
		if err != nil {
			return fmt.Errorf("open apk entry %s failed: %w", file.Name, err)
		}
		dst, err := os.OpenFile(filepath.Join(stagingDir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			_ = src.Close()
			return fmt.Errorf("stage apk entry %s failed: %w", file.Name, err)
		}
		_, copyErr := io.Copy(dst, src)
		closeErr := dst.Close()
		_ = src.Close()
		if copyErr != nil {
			return fmt.Errorf("extract apk entry %s failed: %w", file.Name, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close staged apk entry %s failed: %w", file.Name, closeErr)
		}
		delete(required, file.Name)
	}
	if len(required) != 0 {
		missing := make([]string, 0, len(required))
		for key := range required {
			missing = append(missing, key)
		}
		sort.Strings(missing)
		return fmt.Errorf("missing files in apk: %v", missing)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create destination dir failed: %w", err)
	}
	for _, name := range names {
		if err := os.Rename(filepath.Join(stagingDir, name), filepath.Join(destDir, name)); err != nil {
			return fmt.Errorf("install library %s failed: %w", name, err)
		}
	}
	return nil
}
