package manager

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitxeno/atvloadly/internal/app"
)

func envValue(env []string, key string) string {
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			return v
		}
	}
	return ""
}

// Debug mode forces the signing engine to log at debug level through RUST_LOG,
// overriding a RUST_LOG already present in the process environment.
func TestGetRunEnvsRustLogDebug(t *testing.T) {
	previous := app.Settings
	t.Cleanup(func() { app.Settings = previous })
	app.Settings = &app.SettingsConfiguration{}

	t.Setenv("RUST_LOG", "info")
	if got := envValue(GetRunEnvs(), "RUST_LOG"); got != "info" {
		t.Fatalf("RUST_LOG = %q without debug mode, want the inherited info", got)
	}

	app.Settings.Advanced.DebugMode = true
	if got := envValue(GetRunEnvs(), "RUST_LOG"); got != "debug" {
		t.Fatalf("RUST_LOG = %q with debug mode, want debug", got)
	}
}

// An Apple ID installation hands RUST_LOG=debug to the engine only when debug
// mode is enabled, proving the setting reaches the install command's env.
func TestStartAppleIDPassesRustLogDebug(t *testing.T) {
	const udid = "TEST-DEVICE-UDID"
	for _, tt := range []struct {
		name  string
		debug bool
		want  string
	}{
		{name: "debug off", debug: false, want: ""},
		{name: "debug on", debug: true, want: "debug"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			record := filepath.Join(dir, "rustlog")
			script := "#!/bin/sh\n" +
				"if [ \"$1\" = sign ] || [ \"$1\" = sign-rsd ]; then\n" +
				"  printf '%s' \"$RUST_LOG\" > " + record + "\n" +
				"  echo 'Installation Succeeded'\n" +
				"elif [ \"$1\" = check ]; then\n" +
				"  echo 'SUCCESS'\n" +
				"fi\n" +
				"exit 0\n"
			if err := os.WriteFile(filepath.Join(dir, "plumesign"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("RUST_LOG", "")

			setupAppleIDInstall(t, udid)
			app.Settings.Advanced.DebugMode = tt.debug

			mgr := NewInstallManager()
			defer mgr.Close()

			if err := mgr.Start(context.Background(), InstallOptions{
				UDID:    udid,
				Account: "user@example.com",
				IpaPath: "app.ipa",
			}); err != nil {
				t.Fatalf("install failed: %v", err)
			}

			data, err := os.ReadFile(record)
			if err != nil {
				t.Fatalf("the signing engine was not run: %v", err)
			}
			if got := string(data); got != tt.want {
				t.Fatalf("engine RUST_LOG = %q, want %q", got, tt.want)
			}
		})
	}
}
