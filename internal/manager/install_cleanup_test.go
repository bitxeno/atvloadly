package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
)

func TestCleanUploadTempFilesKeepsPlumeStageFiles(t *testing.T) {
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
	stage := filepath.Join(os.TempDir(), fmt.Sprintf("plume_stage_atvloadly_test_%d", time.Now().UnixNano()))
	if err := os.MkdirAll(stage, 0o755); err != nil {
		t.Fatalf("mkdir stage dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(stage) }()

	// A request-rejection cleanup removes only the given upload file; the
	// staging directories of running installations stay untouched.
	CleanUploadTempFiles(uploaded)
	if _, err := os.Stat(uploaded); !os.IsNotExist(err) {
		t.Fatalf("uploaded file still exists: %v", err)
	}
	if _, err := os.Stat(stage); err != nil {
		t.Fatalf("stage dir was removed by CleanUploadTempFiles: %v", err)
	}

	// After an installation finished, its staging leftovers are removed.
	ins := &InstallManager{}
	ins.CleanTempFiles()
	if _, err := os.Stat(stage); !os.IsNotExist(err) {
		t.Fatalf("stage dir still exists after CleanTempFiles: %v", err)
	}
}
