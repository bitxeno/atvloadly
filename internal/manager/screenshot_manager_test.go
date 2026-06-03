package manager

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/model"
)

func TestResolveDeveloperDiskImageUsesDataDirRoot(t *testing.T) {
	dataDir := t.TempDir()
	app.Config = &app.Configuration{}
	app.Config.Server.DataDir = dataDir

	wantDir := filepath.Join(dataDir, "DeveloperDiskImages", "tvOS_DDI")
	if err := os.MkdirAll(wantDir, 0o755); err != nil {
		t.Fatalf("mkdir ddi dir: %v", err)
	}
	for _, name := range []string{"Image.dmg", "BuildManifest.plist", "Image.dmg.trustcache"} {
		if err := os.WriteFile(filepath.Join(wantDir, name), []byte(name), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	dev := &model.Device{DeviceClass: string(model.DeviceClassAppleTV)}
	dmgPath, manifestPath, trustCachePath, err := resolveDeveloperDiskImage(dev)
	if err != nil {
		t.Fatalf("resolveDeveloperDiskImage returned error: %v", err)
	}

	if dmgPath != filepath.Join(wantDir, "Image.dmg") {
		t.Fatalf("dmgPath = %q, want %q", dmgPath, filepath.Join(wantDir, "Image.dmg"))
	}
	if manifestPath != filepath.Join(wantDir, "BuildManifest.plist") {
		t.Fatalf("manifestPath = %q, want %q", manifestPath, filepath.Join(wantDir, "BuildManifest.plist"))
	}
	if trustCachePath != filepath.Join(wantDir, "Image.dmg.trustcache") {
		t.Fatalf("trustCachePath = %q, want %q", trustCachePath, filepath.Join(wantDir, "Image.dmg.trustcache"))
	}
}

func TestResolveDeveloperDiskImageDownloadsMissingFiles(t *testing.T) {
	dataDir := t.TempDir()
	app.Config = &app.Configuration{}
	app.Config.Server.DataDir = dataDir

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tvOS_DDI/Image.dmg", "/tvOS_DDI/BuildManifest.plist", "/tvOS_DDI/Image.dmg.trustcache":
			_, _ = w.Write([]byte(filepath.Base(r.URL.Path)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	originalBaseURL := developerDiskImageRepoBaseURL
	developerDiskImageRepoBaseURL = server.URL
	t.Cleanup(func() {
		developerDiskImageRepoBaseURL = originalBaseURL
	})

	dev := &model.Device{DeviceClass: string(model.DeviceClassAppleTV)}
	dmgPath, manifestPath, trustCachePath, err := resolveDeveloperDiskImage(dev)
	if err != nil {
		t.Fatalf("resolveDeveloperDiskImage returned error: %v", err)
	}

	for _, path := range []string{dmgPath, manifestPath, trustCachePath} {
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("expected recovered file %q to exist: %v", path, statErr)
		}
	}
}
