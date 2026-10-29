package service

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeAPK(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "applemusic.apk")
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(out)
	for name, contents := range files {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInstallCoreADILibraries(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		contents                 map[string]string
		wantError                bool
		expectedOne, expectedTwo string
	}{
		{"valid apk", map[string]string{"lib/x86_64/libstoreservicescore.so": "new-stores", "lib/x86_64/libCoreADI.so": "new-core"}, false, "new-stores", "new-core"},
		{"missing library", map[string]string{"lib/x86_64/libstoreservicescore.so": "new-stores"}, true, "old-stores", "old-core"},
		{"wrong architecture", map[string]string{"lib/arm64-v8a/libstoreservicescore.so": "new-stores", "lib/arm64-v8a/libCoreADI.so": "new-core"}, true, "old-stores", "old-core"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "x86_64")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			for name, value := range map[string]string{"libstoreservicescore.so": "old-stores", "libCoreADI.so": "old-core"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			err := installCoreADILibraries(fakeAPK(t, tc.contents), dir, "x86_64")
			if (err != nil) != tc.wantError {
				t.Fatalf("err=%v, wantError=%v", err, tc.wantError)
			}
			for name, want := range map[string]string{"libstoreservicescore.so": tc.expectedOne, "libCoreADI.so": tc.expectedTwo} {
				got, e := os.ReadFile(filepath.Join(dir, name))
				if e != nil || string(got) != want {
					t.Fatalf("%s: %s / %v; want %s", name, got, e, want)
				}
			}
			entries, e := os.ReadDir(filepath.Dir(dir))
			if e != nil {
				t.Fatal(e)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".coreadi-") {
					t.Fatalf("staging directory left behind: %s", entry.Name())
				}
			}
		})
	}
}

func TestCorruptAPKPreservesOriginal(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "x86_64")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "libstoreservicescore.so")
	if err := os.WriteFile(old, []byte("old-stores"), 0o644); err != nil {
		t.Fatal(err)
	}
	invalid := filepath.Join(t.TempDir(), "invalid.apk")
	if err := os.WriteFile(invalid, []byte("bad zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installCoreADILibraries(invalid, dir, "x86_64"); err == nil {
		t.Fatal("expected invalid archive to fail")
	}
	if got, _ := os.ReadFile(old); string(got) != "old-stores" {
		t.Fatalf("old library modified: %s", got)
	}
}

func TestBadSecondLibraryPreservesOriginals(t *testing.T) {
	first := "lib/x86_64/libstoreservicescore.so"
	second := "lib/x86_64/libCoreADI.so"
	archive := filepath.Join(t.TempDir(), "corrupt.apk")
	out, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(out)
	for _, file := range []struct{ name, data string }{{first, "first-library-good"}, {second, "second-library-corrupt"}} {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: file.name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(file.data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	off := strings.Index(string(bytes), "second-library-corrupt")
	if off < 0 {
		t.Fatal("second library not found in fixture")
	}
	bytes[off] = 'X' // ZIP structure intact; CRC must fail for second file
	if err := os.WriteFile(archive, bytes, 0o600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "lib", "x86_64")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"libstoreservicescore.so": "old-one", "libCoreADI.so": "old-two"} {
		if err := os.WriteFile(filepath.Join(dest, name), []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := installCoreADILibraries(archive, dest, "x86_64"); err == nil {
		t.Fatal("want CRC error")
	}
	for name, value := range map[string]string{"libstoreservicescore.so": "old-one", "libCoreADI.so": "old-two"} {
		b, err := os.ReadFile(filepath.Join(dest, name))
		if err != nil || string(b) != value {
			t.Fatalf("%s changed to %q (%v)", name, b, err)
		}
	}
}
