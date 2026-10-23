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

// The engine reports the active anisette server on stdout with a log prefix;
// the manager picks the URL out of that line.
func TestAnisetteServerReadsEngineOutput(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"printf '[2026-10-23 12:32:34 INFO  plumesign::commands::anisette] Anisette server: https://ani.example.com (source: anisette.json)\\n'\n"
	if err := os.WriteFile(filepath.Join(dir, "plumesign"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	state, err := AnisetteServer()
	if err != nil {
		t.Fatalf("AnisetteServer() error = %v", err)
	}
	if state.Server != "https://ani.example.com" {
		t.Errorf("server = %q, want the URL without the log prefix and source suffix", state.Server)
	}
	if state.IsDefault {
		t.Error("a configured server was reported as the default")
	}
}

// A default engine server reads back as empty, because an unset input means
// "use the default" in the settings UI.
func TestAnisetteServerReadsDefaultAsEmpty(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"printf '[2026-10-23 12:32:34 INFO  plumesign::commands::anisette] Anisette server: https://mac-ani.jkcoxson.com (source: default)\\n'\n"
	if err := os.WriteFile(filepath.Join(dir, "plumesign"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	state, err := AnisetteServer()
	if err != nil {
		t.Fatalf("AnisetteServer() error = %v", err)
	}
	if state.Server != "" {
		t.Errorf("server = %q, want empty for the engine default", state.Server)
	}
	if !state.IsDefault {
		t.Error("the default server was not reported as default")
	}
}

// A custom server reaches the engine as the value of --set.
func TestSetAnisetteServerRunsEngineCommand(t *testing.T) {
	record := fakeAnisetteEngine(t, "0")

	if err := SetAnisetteServer("https://ani.example.com"); err != nil {
		t.Fatalf("SetAnisetteServer() error = %v", err)
	}

	want := "anisette server --set https://ani.example.com"
	if got := strings.Join(recordedArgs(t, record), " "); got != want {
		t.Errorf("arguments = %q, want %q", got, want)
	}
}

// An empty server restores the engine default through --reset instead of
// passing an empty --set value.
func TestSetAnisetteServerEmptyRestoresDefault(t *testing.T) {
	record := fakeAnisetteEngine(t, "0")

	if err := SetAnisetteServer("   "); err != nil {
		t.Fatalf("SetAnisetteServer() error = %v", err)
	}

	want := "anisette server --reset"
	if got := strings.Join(recordedArgs(t, record), " "); got != want {
		t.Errorf("arguments = %q, want %q", got, want)
	}
}

// A value the engine would reject is refused before a command runs.
func TestSetAnisetteServerRejectsInvalidURL(t *testing.T) {
	record := fakeAnisetteEngine(t, "0")

	if err := SetAnisetteServer("notaurl"); err == nil {
		t.Fatal("SetAnisetteServer() error = nil, want an error")
	}

	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Errorf("the engine was run for an invalid URL (stat err = %v)", err)
	}
}
