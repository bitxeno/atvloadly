package manager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitxeno/atvloadly/internal/app"
)

func TestCleanTempFilesKeepsOtherPlumeStageFiles(t *testing.T) {
	dataDir := t.TempDir()
	app.Config = &app.Configuration{}
	app.Config.Server.DataDir = dataDir

	tempDir := filepath.Join(dataDir, "tmp")
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		t.Fatalf("mkdir tmp dir: %v", err)
	}
	uploaded := filepath.Join(tempDir, "app.ipa")
	if err := os.WriteFile(uploaded, []byte("ipa"), 0o644); err != nil {
		t.Fatalf("write %s: %v", uploaded, err)
	}

	// This directory represents a different plumesign process that is still
	// running. CleanTempFiles must never remove staging it does not own.
	stage, err := os.MkdirTemp("", "plume_stage_atvloadly_concurrent_*")
	if err != nil {
		t.Fatalf("mkdir plume stage: %v", err)
	}
	defer func() { _ = os.RemoveAll(stage) }()

	ins := &InstallManager{}
	ins.CleanTempFiles(uploaded)

	if _, err := os.Stat(uploaded); !os.IsNotExist(err) {
		t.Fatalf("uploaded file still exists: %v", err)
	}
	if _, err := os.Stat(stage); err != nil {
		t.Fatalf("another install's stage dir was removed: %v", err)
	}
}

func TestAppleIDRunEnvUsesPrivateTempDir(t *testing.T) {
	base := []string{
		"HOME=/shared-home",
		"TMPDIR=/shared-tmp",
		"PATH=/usr/bin",
	}

	env, tempDir, err := appleIDRunEnv(base)
	if err != nil {
		t.Fatalf("appleIDRunEnv: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	info, err := os.Stat(tempDir)
	if err != nil {
		t.Fatalf("private temp dir missing: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("private temp path is not a directory: %s", tempDir)
	}

	values := make(map[string][]string)
	for _, kv := range env {
		key, value, ok := strings.Cut(kv, "=")
		if ok {
			values[key] = append(values[key], value)
		}
	}

	if got := values["TMPDIR"]; len(got) != 1 || got[0] != tempDir {
		t.Fatalf("TMPDIR = %v, want only %q", got, tempDir)
	}
	if got := values["HOME"]; len(got) != 1 || got[0] != "/shared-home" {
		t.Fatalf("HOME = %v, want shared Apple ID home", got)
	}
}
