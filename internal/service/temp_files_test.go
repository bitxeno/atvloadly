package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bitxeno/atvloadly/internal/manager"
)

func setModTime(t *testing.T, path string, age time.Duration) {
	t.Helper()
	modTime := time.Now().Add(-age)
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveStaleTempFiles(t *testing.T) {
	dataDir := setTestDataDir(t)
	tmpDir := filepath.Join(dataDir, "tmp")

	staleIPA := writeTestFile(t, filepath.Join(tmpDir, "install_url_1.ipa"))
	setModTime(t, staleIPA, 25*time.Hour)
	staleIcon := writeTestFile(t, filepath.Join(tmpDir, "install_url_1.png"))
	setModTime(t, staleIcon, 48*time.Hour)
	recent := writeTestFile(t, filepath.Join(tmpDir, "app_2.ipa"))
	setModTime(t, recent, 23*time.Hour)

	nested := writeTestFile(t, filepath.Join(tmpDir, "sub", "app_3.ipa"))
	setModTime(t, nested, 48*time.Hour)
	subDir := filepath.Join(tmpDir, "sub")
	setModTime(t, subDir, 48*time.Hour)

	// The link itself is recent; its target is an old file outside tmp.
	target := writeTestFile(t, filepath.Join(t.TempDir(), "outside.ipa"))
	setModTime(t, target, 48*time.Hour)
	link := filepath.Join(tmpDir, "link.ipa")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	removed, err := RemoveStaleTempFiles()
	if err != nil {
		t.Fatalf("RemoveStaleTempFiles: %v", err)
	}
	if removed != 2 {
		t.Errorf("removed %d files, want 2", removed)
	}
	for _, path := range []string{staleIPA, staleIcon} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("stale file %s was kept: %v", path, err)
		}
	}
	for _, path := range []string{recent, subDir, nested, link, target} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("%s was removed: %v", path, err)
		}
	}
}

func TestRemoveStaleTempFilesMissingDirectory(t *testing.T) {
	dataDir := setTestDataDir(t)
	if err := os.Remove(filepath.Join(dataDir, "tmp")); err != nil {
		t.Fatal(err)
	}

	removed, err := RemoveStaleTempFiles()
	if err != nil || removed != 0 {
		t.Fatalf("RemoveStaleTempFiles = %d, %v; want 0, nil", removed, err)
	}
}

func TestRemoveStaleTempFilesKeepsReservedFiles(t *testing.T) {
	dataDir := setTestDataDir(t)
	tmpDir := filepath.Join(dataDir, "tmp")

	queued := writeTestFile(t, filepath.Join(tmpDir, "queued_app.ipa"))
	setModTime(t, queued, 48*time.Hour)

	// A queued installation still owns its upload even after the abandoned
	// age: the sweep keeps it until the task releases it.
	manager.ReserveUploadTempFiles(queued)
	if removed, err := RemoveStaleTempFiles(); err != nil || removed != 0 {
		t.Fatalf("RemoveStaleTempFiles = %d, %v; want 0, nil", removed, err)
	}
	if _, err := os.Lstat(queued); err != nil {
		t.Fatalf("reserved file was removed: %v", err)
	}

	manager.ReleaseUploadTempFiles(queued)
	if removed, err := RemoveStaleTempFiles(); err != nil || removed != 1 {
		t.Fatalf("RemoveStaleTempFiles = %d, %v; want 1, nil", removed, err)
	}
	if _, err := os.Lstat(queued); !os.IsNotExist(err) {
		t.Fatalf("released file was kept: %v", err)
	}
}
