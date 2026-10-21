//go:build darwin

package cfg

import (
	"path/filepath"
	"testing"
)

func TestDefaultConfigDirIsStable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	want := filepath.Join(home, "Library", "Application Support", "atvloadly")
	if got := DefaultConfigDir(); got != want {
		t.Fatalf("DefaultConfigDir = %q, want %q", got, want)
	}
}
