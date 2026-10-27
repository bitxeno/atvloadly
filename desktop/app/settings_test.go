package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureDesktopConfigCreatesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings", "config.yaml")

	if err := ensureDesktopConfig(path); err != nil {
		t.Fatalf("ensureDesktopConfig() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "port: 5533") || !strings.Contains(string(data), "listen_addr: 127.0.0.1") {
		t.Errorf("config = %q, want desktop port and loopback listen address", data)
	}
}

func TestEnsureDesktopConfigKeepsExistingValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("server:\n  port: 12345\n  listen_addr: 0.0.0.0\n  work_dir: /custom/data\nlog:\n  level: debug\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := ensureDesktopConfig(path); err != nil {
		t.Fatalf("ensureDesktopConfig() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != string(original) {
		t.Errorf("config was modified:\n%s", data)
	}
}

func TestLoadDesktopSettingsDefaultsAndOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	settings, err := loadDesktopSettings(path)
	if err != nil {
		t.Fatalf("loadDesktopSettings() error = %v", err)
	}
	if settings.Port != defaultServerPort || settings.ListenAddr != loopbackListenAddr {
		t.Errorf("defaults = %+v", settings)
	}

	if err := saveDesktopSettings(path, desktopSettings{Port: 12345, ListenAddr: lanListenAddr}); err != nil {
		t.Fatalf("saveDesktopSettings() error = %v", err)
	}
	settings, err = loadDesktopSettings(path)
	if err != nil {
		t.Fatalf("loadDesktopSettings() error = %v", err)
	}
	if settings.Port != 12345 || !settings.allowLAN() {
		t.Errorf("saved settings = %+v, want port 12345 with LAN", settings)
	}
}

func TestSaveDesktopSettingsPreservesOtherKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("server:\n  port: 5533\n  listen_addr: 127.0.0.1\n  work_dir: /custom/data\nlog:\n  level: debug\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := saveDesktopSettings(path, desktopSettings{Port: 12345, ListenAddr: lanListenAddr}); err != nil {
		t.Fatalf("saveDesktopSettings() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	for _, want := range []string{"port: 12345", "listen_addr: 0.0.0.0", "work_dir: /custom/data", "level: debug"} {
		if !strings.Contains(got, want) {
			t.Errorf("config missing %q:\n%s", want, got)
		}
	}
}

func TestValidateDesktopSettings(t *testing.T) {
	for _, port := range []int{0, -1, 65536} {
		if err := validateServerPort(port); !errors.Is(err, errInvalidServerPort) {
			t.Errorf("validateServerPort(%d) = %v, want invalid port", port, err)
		}
	}
	for _, port := range []int{1, 5533, 65535} {
		if err := validateServerPort(port); err != nil {
			t.Errorf("validateServerPort(%d) = %v", port, err)
		}
	}
	if err := validateListenAddr("192.168.1.5"); err == nil {
		t.Error("validateListenAddr accepted an arbitrary address")
	}
	for _, addr := range []string{loopbackListenAddr, lanListenAddr} {
		if err := validateListenAddr(addr); err != nil {
			t.Errorf("validateListenAddr(%q) = %v", addr, err)
		}
	}
}
