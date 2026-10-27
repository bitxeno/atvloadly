package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestEnsureDesktopConfigCreatesDefaultPort(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings", "config.yaml")

	if err := ensureDesktopConfig(path); err != nil {
		t.Fatalf("ensureDesktopConfig() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "server:\n  port: 5533\n" {
		t.Errorf("config = %q, want default server.port", data)
	}
}

func TestEnsureDesktopConfigAddsPortAndKeepsExistingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("server:\n  work_dir: /custom/data\nlog:\n  level: debug\n")
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
	if string(data) == string(original) {
		t.Fatal("config was not updated with default port")
	}
	if !strings.Contains(string(data), "port: 5533") || !strings.Contains(string(data), "work_dir: /custom/data") || !strings.Contains(string(data), "level: debug") {
		t.Errorf("config did not preserve existing values and add port:\n%s", data)
	}
}

func TestEnsureDesktopConfigPreservesConfiguredPort(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("server:\n  port: 12345\n  work_dir: /custom/data\n")
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
		t.Errorf("configured port was modified:\n%s", data)
	}
}

func TestEnsureDesktopConfigRejectsInvalidPort(t *testing.T) {
	for _, port := range []int{0, -1, 65536} {
		t.Run(strconv.Itoa(port), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			data := []byte("server:\n  port: " + strconv.Itoa(port) + "\n")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			err := ensureDesktopConfig(path)
			if !errors.Is(err, errInvalidServerPort) {
				t.Errorf("ensureDesktopConfig() error = %v, want invalid port error", err)
			}
		})
	}
}

func TestSetServerPortUpdatesExistingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("server:\n  port: 5533\n  work_dir: /custom/data\nlog:\n  level: debug\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := setServerPort(path, 12345); err != nil {
		t.Fatalf("setServerPort() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "port: 12345") || !strings.Contains(string(data), "work_dir: /custom/data") || !strings.Contains(string(data), "level: debug") {
		t.Errorf("config did not update port and preserve other values:\n%s", data)
	}
}

func TestSetServerPortRejectsInvalidPortWithoutChangingConfig(t *testing.T) {
	for _, port := range []int{0, -1, 65536} {
		t.Run(strconv.Itoa(port), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			original := []byte("server:\n  port: 5533\n")
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			err := setServerPort(path, port)
			if !errors.Is(err, errInvalidServerPort) {
				t.Fatalf("setServerPort() error = %v, want invalid port error", err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			if string(data) != string(original) {
				t.Errorf("config changed for invalid port:\n%s", data)
			}
		})
	}
}
