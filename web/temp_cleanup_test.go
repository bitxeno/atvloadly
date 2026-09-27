package web

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveUploadTempFile(t *testing.T) {
	dataDir := t.TempDir()
	tmpDir := filepath.Join(dataDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeFile := func(path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("removes direct upload temp file", func(t *testing.T) {
		path := filepath.Join(tmpDir, "upload.ipa")
		writeFile(path)

		if err := removeUploadTempFile(dataDir, path); err != nil {
			t.Fatalf("removeUploadTempFile() error = %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("temp file still exists, stat error = %v", err)
		}
	})

	t.Run("allows already removed upload temp file", func(t *testing.T) {
		path := filepath.Join(tmpDir, "missing.ipa")
		if err := removeUploadTempFile(dataDir, path); err != nil {
			t.Fatalf("removeUploadTempFile() error = %v", err)
		}
	})

	t.Run("rejects file outside upload temp directory", func(t *testing.T) {
		path := filepath.Join(dataDir, "config.yaml")
		writeFile(path)

		if err := removeUploadTempFile(dataDir, path); err == nil {
			t.Fatal("removeUploadTempFile() accepted path outside upload temp directory")
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("outside file was changed: %v", err)
		}
	})

	t.Run("rejects upload temp directory itself", func(t *testing.T) {
		if err := removeUploadTempFile(dataDir, tmpDir); err == nil {
			t.Fatal("removeUploadTempFile() accepted upload temp directory")
		}
		if info, err := os.Stat(tmpDir); err != nil || !info.IsDir() {
			t.Fatalf("upload temp directory was changed: info=%v err=%v", info, err)
		}
	})

	t.Run("rejects nested path", func(t *testing.T) {
		path := filepath.Join(tmpDir, "nested", "icon.png")
		writeFile(path)

		if err := removeUploadTempFile(dataDir, path); err == nil {
			t.Fatal("removeUploadTempFile() accepted nested path")
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("nested file was changed: %v", err)
		}
	})

	t.Run("rejects sibling with temp prefix", func(t *testing.T) {
		path := filepath.Join(dataDir, "tmp-other", "upload.ipa")
		writeFile(path)

		if err := removeUploadTempFile(dataDir, path); err == nil {
			t.Fatal("removeUploadTempFile() accepted sibling directory")
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("sibling file was changed: %v", err)
		}
	})
}
