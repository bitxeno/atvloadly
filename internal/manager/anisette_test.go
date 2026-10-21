package manager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeAnisetteEngine puts a plumesign on PATH that records the arguments of the
// anisette command it is given and exits with exitCode.
func fakeAnisetteEngine(t *testing.T, exitCode string) string {
	t.Helper()
	dir := t.TempDir()
	record := filepath.Join(dir, "args")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + record + "\n" +
		"exit " + exitCode + "\n"
	if err := os.WriteFile(filepath.Join(dir, "plumesign"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return record
}

// Anisette data is reset by the engine rather than by deleting files this
// process guesses, so the engine owns the location of its provisioning data.
func TestResetAnisetteRunsEngineCommand(t *testing.T) {
	record := fakeAnisetteEngine(t, "0")

	if err := ResetAnisette(); err != nil {
		t.Fatalf("ResetAnisette() error = %v", err)
	}

	if got := strings.Join(recordedArgs(t, record), " "); got != "anisette reset" {
		t.Errorf("arguments = %q, want %q", got, "anisette reset")
	}
}

// A failing engine surfaces its error instead of reporting a successful reset.
func TestResetAnisetteReportsEngineFailure(t *testing.T) {
	fakeAnisetteEngine(t, "1")

	if err := ResetAnisette(); err == nil {
		t.Fatal("ResetAnisette() error = nil, want an error")
	}
}
