package main

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"
)

// plumesignBinDir returns the directory of the plumesign binary shipped in
// the app's resources, or an empty string when the app runs unpackaged (go
// run, go test): then plumesign comes from the PATH, as it does for the
// server command.
func plumesignBinDir() string {
	packaged := mygo.App.IsPackaged()
	dir, err := mygo.App.Path(mygo.PathResources)
	if err != nil || !packaged {
		return ""
	}
	for _, name := range plumesignNames() {
		if _, err := os.Stat(filepath.Join(dir, "bin", name)); err == nil {
			return filepath.Join(dir, "bin")
		}
	}
	return ""
}

func plumesignNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"plumesign.exe", "plumesign"}
	}
	return []string{"plumesign"}
}
