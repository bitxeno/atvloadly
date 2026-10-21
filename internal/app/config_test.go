package app

import (
	"path/filepath"
	"testing"
)

func TestSideloadDirectoriesMatchEngine(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	if got, want := SideloadDataDir(), filepath.Join(home, ".config", "PlumeImpactor"); got != want {
		t.Fatalf("SideloadDataDir = %q, want %q", got, want)
	}

	xdg := filepath.Join(t.TempDir(), "config with spaces")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if got, want := SideloadDataDir(), filepath.Join(xdg, "PlumeImpactor"); got != want {
		t.Fatalf("SideloadDataDir = %q, want %q", got, want)
	}
	if got, want := RemotePairingDir(), filepath.Join(xdg, "PlumeImpactor", "pairing_files"); got != want {
		t.Fatalf("RemotePairingDir = %q, want %q", got, want)
	}
}
