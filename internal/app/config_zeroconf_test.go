package app

import (
	"os"
	"path/filepath"
	"testing"
)

// writeConfig writes a config.yaml with the given body into a temp dir and
// returns its path.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// TestZeroconfEnabledByDefault verifies discovery stays on when the config
// file does not mention it.
func TestZeroconfEnabledByDefault(t *testing.T) {
	path := writeConfig(t, "server:\n  work_dir: /tmp\n")
	if _, err := InitConfig(path, false); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}
	if !Config.Zeroconf || !ZeroconfEnabled() {
		t.Fatal("zeroconf should be enabled by default")
	}
}

// TestZeroconfDisabledByConfig verifies zeroconf: false in the config file
// disables discovery.
func TestZeroconfDisabledByConfig(t *testing.T) {
	path := writeConfig(t, "zeroconf: false\nserver:\n  work_dir: /tmp\n")
	if _, err := InitConfig(path, false); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}
	if Config.Zeroconf || ZeroconfEnabled() {
		t.Fatal("zeroconf should be disabled by the config file")
	}
}

// TestZeroconfEnvOverridesConfig verifies ZEROCONF_ENABLED overrides the config
// file value.
func TestZeroconfEnvOverridesConfig(t *testing.T) {
	t.Setenv("ZEROCONF_ENABLED", "true")
	path := writeConfig(t, "zeroconf: false\nserver:\n  work_dir: /tmp\n")
	if _, err := InitConfig(path, false); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}
	if !Config.Zeroconf {
		t.Fatal("ZEROCONF_ENABLED=true should override zeroconf: false")
	}
}

// TestZeroconfInvalidEnvIgnored verifies an unparseable ZEROCONF_ENABLED keeps
// the configured value instead of silently flipping discovery off.
func TestZeroconfInvalidEnvIgnored(t *testing.T) {
	t.Setenv("ZEROCONF_ENABLED", "not-a-bool")
	path := writeConfig(t, "zeroconf: true\nserver:\n  work_dir: /tmp\n")
	if _, err := InitConfig(path, false); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}
	if !Config.Zeroconf {
		t.Fatal("invalid ZEROCONF_ENABLED should keep the configured value")
	}
}
