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

// Anisette state is provisioned per account, so the reset names the account
// and the engine discards exactly that state.
func TestResetAnisetteRunsEngineCommandForAccount(t *testing.T) {
	record := fakeAnisetteEngine(t, "0")

	if err := ResetAnisette("user@example.com"); err != nil {
		t.Fatalf("ResetAnisette() error = %v", err)
	}

	want := "anisette reset -u user@example.com"
	if got := strings.Join(recordedArgs(t, record), " "); got != want {
		t.Errorf("arguments = %q, want %q", got, want)
	}
}

// A blank account names no Anisette state, so the call is rejected before any
// command runs.
func TestResetAnisetteRequiresAccount(t *testing.T) {
	record := fakeAnisetteEngine(t, "0")

	if err := ResetAnisette("   "); err == nil {
		t.Fatal("ResetAnisette() error = nil, want an error")
	}

	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Errorf("the engine was run for a blank account (stat err = %v)", err)
	}
}

// A failing engine surfaces its error instead of reporting a successful reset.
func TestResetAnisetteReportsEngineFailure(t *testing.T) {
	fakeAnisetteEngine(t, "1")

	if err := ResetAnisette("user@example.com"); err == nil {
		t.Fatal("ResetAnisette() error = nil, want an error")
	}
}
