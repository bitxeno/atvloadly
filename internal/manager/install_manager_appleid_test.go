package manager

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/model"
)

// fakeAppleIDEngine puts a plumesign on PATH that records the arguments of the
// sign command it is given and reports a successful installation. Every other
// subcommand (check afc, device-info) succeeds with no output.
func fakeAppleIDEngine(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	record := filepath.Join(dir, "args")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = sign ] || [ \"$1\" = sign-rsd ]; then\n" +
		"  printf '%s\\n' \"$@\" > " + record + "\n" +
		"  echo 'Installation Succeeded'\n" +
		"elif [ \"$1\" = check ]; then\n" +
		"  echo 'SUCCESS'\n" +
		"fi\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "plumesign"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return record
}

// setupAppleIDInstall prepares the configuration and the device registry an
// Apple ID installation reads.
func setupAppleIDInstall(t *testing.T, udid string) {
	t.Helper()
	previousConfig, previousSettings := app.Config, app.Settings
	app.Config = &app.Configuration{}
	app.Config.Server.DataDir = t.TempDir()
	app.Settings = &app.SettingsConfiguration{}
	t.Cleanup(func() { app.Config, app.Settings = previousConfig, previousSettings })

	deviceManager.devices.Store(udid, model.Device{UDID: udid, Name: "Apple TV", DeviceClass: "AppleTV"})
	t.Cleanup(func() { deviceManager.devices.Delete(udid) })
}

// An Apple ID installation passes the custom bundle identifier to the engine,
// and a refresh passes it again: the stored IPA still carries the bundle
// identifier of the original package, so without it the app would be refreshed
// under another App ID.
func TestStartAppleIDPassesCustomIdentifier(t *testing.T) {
	const udid = "TEST-DEVICE-UDID"
	for _, tt := range []struct {
		name        string
		refresh     bool
		custom      string
		wantFlag    string
		wantRefresh bool
	}{
		{name: "install with custom identifier", custom: "app.custom", wantFlag: "--custom-identifier=app.custom"},
		{name: "refresh with custom identifier", custom: "app.custom", refresh: true, wantFlag: "--custom-identifier=app.custom", wantRefresh: true},
		{name: "install without custom identifier"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			record := fakeAppleIDEngine(t)
			setupAppleIDInstall(t, udid)

			mgr := NewInstallManager()
			defer mgr.Close()

			err := mgr.Start(context.Background(), InstallOptions{
				UDID:             udid,
				Account:          "user@example.com",
				IpaPath:          "app.ipa",
				CustomIdentifier: tt.custom,
				RefreshMode:      tt.refresh,
			})
			if err != nil {
				t.Fatalf("install failed: %v", err)
			}

			data, err := os.ReadFile(record)
			if err != nil {
				t.Fatalf("the signing engine was not run: %v", err)
			}
			args := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
			joined := strings.Join(args, " ")

			if tt.wantFlag == "" {
				if strings.Contains(joined, "--custom-identifier") {
					t.Fatalf("no custom identifier was requested but the engine got %q", joined)
				}
			} else if !containsArg(args, tt.wantFlag) {
				t.Fatalf("engine args %q do not contain %q", joined, tt.wantFlag)
			}
			if gotRefresh := containsArg(args, "--refresh"); gotRefresh != tt.wantRefresh {
				t.Fatalf("--refresh present = %v, want %v (%q)", gotRefresh, tt.wantRefresh, joined)
			}
		})
	}
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

// The process exit status is the completion contract. INFO logs may be
// suppressed by RUST_LOG and a success-looking log can precede a later failure.
func TestAppleIDCompletionUsesExitStatus(t *testing.T) {
	for _, tt := range []struct {
		name      string
		output    string
		exitCode  string
		wantError bool
	}{
		{name: "silent success", exitCode: "0"},
		{name: "failure after a success log", output: "Installation complete!", exitCode: "3", wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupAppleIDInstall(t, "silent-install")
			dir := t.TempDir()
			script := "#!/bin/sh\nif [ \"$1\" = check ]; then echo SUCCESS; exit 0; fi\n" +
				"printf '%s' '" + tt.output + "'\nexit " + tt.exitCode + "\n"
			if err := os.WriteFile(filepath.Join(dir, "plumesign"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("RUST_LOG", "warn")
			mgr := NewInteractiveInstallManager()
			defer mgr.Close()
			err := mgr.Start(context.Background(), InstallOptions{UDID: "silent-install", Account: "test@example.com", IpaPath: "app.ipa"})
			if (err != nil) != tt.wantError {
				t.Fatalf("Start returned %v, want error = %v", err, tt.wantError)
			}
			if !tt.wantError && mgr.OutputLog() != "" {
				t.Fatal("silent engine unexpectedly produced output")
			}
		})
	}
}
