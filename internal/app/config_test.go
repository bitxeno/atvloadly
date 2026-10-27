package app

import (
	"path/filepath"
	"runtime"
	"testing"
)

// TestSideloadDataDirMatchesPlumesign pins the directory to the one plumesign
// itself resolves (get_data_path): %APPDATA%\PlumeImpactor on Windows and
// $XDG_CONFIG_HOME/PlumeImpactor elsewhere. atvloadly reads the pairing files
// plumesign writes, so a mismatch leaves paired devices undiscovered.
func TestSideloadDataDirMatchesPlumesign(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	if runtime.GOOS == "windows" {
		appData := filepath.Join(dir, "AppData", "Roaming")
		t.Setenv("APPDATA", appData)
		want := filepath.Join(appData, "PlumeImpactor")
		if got := SideloadDataDir(); got != want {
			t.Fatalf("SideloadDataDir() = %q, want %q", got, want)
		}
		return
	}

	xdg := filepath.Join(dir, "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	want := filepath.Join(xdg, "PlumeImpactor")
	if got := SideloadDataDir(); got != want {
		t.Fatalf("SideloadDataDir() = %q, want %q", got, want)
	}
}

// TestSideloadDataDirFallsBackToHome covers the unset-variable path: plumesign
// falls back to ~/.config on non-Windows, and to the user's Roaming profile on
// Windows.
func TestSideloadDataDirFallsBackToHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("APPDATA", "")

	want := filepath.Join(dir, ".config", "PlumeImpactor")
	if runtime.GOOS == "windows" {
		want = filepath.Join(dir, "AppData", "Roaming", "PlumeImpactor")
	}
	if got := SideloadDataDir(); got != want {
		t.Fatalf("SideloadDataDir() = %q, want %q", got, want)
	}
}
