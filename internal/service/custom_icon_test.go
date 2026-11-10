package service

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/signing"
)

// testPNG returns a small valid PNG that stands in for an uploaded icon.
func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	img.Set(8, 8, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func setupCustomIconConfig(t *testing.T) string {
	t.Helper()
	previous := app.Config
	dataDir := t.TempDir()
	app.Config = &app.Configuration{}
	app.Config.Server.DataDir = dataDir
	t.Cleanup(func() { app.Config = previous })
	return dataDir
}

func TestSaveCustomIconUploadNormalizesToPNG(t *testing.T) {
	dataDir := setupCustomIconConfig(t)
	path, err := SaveCustomIconUpload(testPNG(t))
	if err != nil {
		t.Fatalf("SaveCustomIconUpload: %v", err)
	}
	if filepath.Dir(path) != filepath.Join(dataDir, "tmp") {
		t.Fatalf("icon saved outside the upload directory: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != "png" || cfg.Width != 16 || cfg.Height != 16 {
		t.Fatalf("stored icon is not the expected PNG: format=%q cfg=%+v err=%v", format, cfg, err)
	}
}

func TestSaveCustomIconUploadRejectsNonImage(t *testing.T) {
	setupCustomIconConfig(t)
	if _, err := SaveCustomIconUpload([]byte("not an image")); err == nil {
		t.Fatal("a non-image upload was accepted")
	} else if signing.CodeOf(err) != signing.CodeCustomIconInvalid {
		t.Fatalf("error code = %q, want %q", signing.CodeOf(err), signing.CodeCustomIconInvalid)
	}
}

// An oversized upload is downscaled to the storage bound while keeping its
// aspect ratio; a small icon is left untouched.
func TestSaveCustomIconUploadDownscalesOversizedImage(t *testing.T) {
	setupCustomIconConfig(t)
	big := image.NewRGBA(image.Rect(0, 0, 2400, 1200))
	var upload bytes.Buffer
	if err := png.Encode(&upload, big); err != nil {
		t.Fatal(err)
	}
	path, err := SaveCustomIconUpload(upload.Bytes())
	if err != nil {
		t.Fatalf("SaveCustomIconUpload: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if format != "png" || cfg.Width != 1024 || cfg.Height != 512 {
		t.Fatalf("stored icon = %dx%d %s, want 1024x512 PNG", cfg.Width, cfg.Height, format)
	}
}

func TestFitCustomIcon(t *testing.T) {
	for _, tt := range []struct {
		name     string
		w, h     int
		wantW    int
		wantH    int
		wantSame bool
	}{
		{name: "small icon unchanged", w: 16, h: 16, wantSame: true},
		{name: "large square", w: 2048, h: 2048, wantW: 1024, wantH: 1024},
		{name: "wide", w: 2400, h: 1200, wantW: 1024, wantH: 512},
		{name: "tall", w: 1200, h: 2400, wantW: 512, wantH: 1024},
	} {
		t.Run(tt.name, func(t *testing.T) {
			src := image.NewRGBA(image.Rect(0, 0, tt.w, tt.h))
			got := fitCustomIcon(src)
			if tt.wantSame {
				if got != src {
					t.Fatal("a small icon was needlessly reallocated")
				}
				return
			}
			cfg := got.Bounds()
			if cfg.Dx() != tt.wantW || cfg.Dy() != tt.wantH {
				t.Fatalf("size = %dx%d, want %dx%d", cfg.Dx(), cfg.Dy(), tt.wantW, tt.wantH)
			}
		})
	}
}

func TestResolveCustomIconPath(t *testing.T) {
	dataDir := setupCustomIconConfig(t)
	uploadDir := filepath.Join(dataDir, "tmp")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		t.Fatal(err)
	}
	valid := filepath.Join(uploadDir, "icon.png")
	if err := os.WriteFile(valid, testPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	invalid := filepath.Join(uploadDir, "broken.png")
	if err := os.WriteFile(invalid, []byte("broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.png")
	if err := os.WriteFile(outside, testPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name    string
		path    string
		wantOK  bool
		wantErr bool
	}{
		{name: "empty keeps the IPA icons", path: "", wantOK: true},
		{name: "uploaded image", path: valid, wantOK: true},
		{name: "unreadable image", path: invalid, wantErr: true},
		{name: "outside the data directory", path: outside, wantErr: true},
		{name: "missing file", path: filepath.Join(uploadDir, "missing.png"), wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveCustomIconPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveCustomIconPath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
			if tt.wantOK && got == "" && tt.path != "" {
				t.Fatalf("a valid icon resolved to an empty path")
			}
			if tt.wantErr && signing.CodeOf(err) != signing.CodeCustomIconInvalid {
				t.Fatalf("error code = %q, want %q", signing.CodeOf(err), signing.CodeCustomIconInvalid)
			}
		})
	}
}

// The displayed icon is the custom one when available, and falls back to the
// IPA icon when there is no custom icon or its file is unreadable.
func TestResolveAppDisplayIconPath(t *testing.T) {
	dataDir := setupCustomIconConfig(t)
	ipaDir := filepath.Join(dataDir, "ipa", "1")
	if err := os.MkdirAll(ipaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(ipaDir, "app.png")
	custom := filepath.Join(ipaDir, customIconFileName)
	for _, p := range []string{original, custom} {
		if err := os.WriteFile(p, testPNG(t), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// resolvePathWithin follows symlinks, so compare against the canonical
	// form of the temp directory (macOS resolves /var to /private/var).
	canonical := func(p string) string {
		resolved, err := filepath.EvalSymlinks(p)
		if err != nil {
			t.Fatal(err)
		}
		return resolved
	}
	original, custom = canonical(original), canonical(custom)

	app := model.InstalledApp{Icon: original}
	if got, err := ResolveAppDisplayIconPath(app); err != nil || got != original {
		t.Fatalf("without a custom icon got %q, err %v; want the IPA icon", got, err)
	}

	app.CustomIconPath = custom
	if got, err := ResolveAppDisplayIconPath(app); err != nil || got != custom {
		t.Fatalf("with a custom icon got %q, err %v; want the custom icon", got, err)
	}

	// A stale custom icon path must not hide the still-valid IPA icon.
	app.CustomIconPath = filepath.Join(ipaDir, "missing.png")
	if got, err := ResolveAppDisplayIconPath(app); err != nil || got != original {
		t.Fatalf("with a missing custom icon got %q, err %v; want the IPA icon", got, err)
	}

	// A custom icon outside the app directory cannot be served either.
	app.CustomIconPath = filepath.Join(t.TempDir(), "evil.png")
	if got, err := ResolveAppDisplayIconPath(app); err != nil || got != original {
		t.Fatalf("with an outside custom icon got %q, err %v; want the IPA icon", got, err)
	}
}

func TestStoreCustomIconPersistsAndClears(t *testing.T) {
	dataDir := setupCustomIconConfig(t)
	uploadDir := filepath.Join(dataDir, "tmp")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		t.Fatal(err)
	}
	uploaded := filepath.Join(uploadDir, "icon.png")
	if err := os.WriteFile(uploaded, testPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	saveDir := filepath.Join(dataDir, "ipa", "7")

	stored := storeCustomIcon(uploaded, saveDir, "")
	if want := filepath.Join(saveDir, customIconFileName); stored != want {
		t.Fatalf("stored path = %q, want %q", stored, want)
	}
	if _, err := os.Stat(stored); err != nil {
		t.Fatalf("the custom icon was not persisted: %v", err)
	}
	// The upload is copied, not moved: another install may still use it.
	if _, err := os.Stat(uploaded); err != nil {
		t.Fatalf("the upload was consumed: %v", err)
	}

	// An empty choice clears the recorded path.
	if got := storeCustomIcon("", saveDir, stored); got != "" {
		t.Fatalf("an empty icon did not clear the choice: %q", got)
	}
	// An invalid path keeps the previous icon rather than losing it.
	if got := storeCustomIcon(filepath.Join(t.TempDir(), "evil.png"), saveDir, stored); got != stored {
		t.Fatalf("an invalid icon replaced the stored icon: %q", got)
	}
}

// A custom icon is stored with the installed app: the refresh and
// source-update flows read the record (which keeps the stored path), so the
// icon survives updates as the feature request asks. An explicitly empty
// choice clears it.
func TestSaveAppPersistsCustomIcon(t *testing.T) {
	setupTestDB(t)
	dataDir := app.Config.Server.DataDir
	uploadDir := filepath.Join(dataDir, "tmp")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		t.Fatal(err)
	}
	uploaded := filepath.Join(uploadDir, "icon.png")
	if err := os.WriteFile(uploaded, testPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	ipaPath := filepath.Join(uploadDir, "app.ipa")
	if err := os.WriteFile(ipaPath, []byte("ipa"), 0o644); err != nil {
		t.Fatal(err)
	}

	first := model.InstalledApp{
		IpaName:          "Test",
		IpaPath:          ipaPath,
		UDID:             "DEVICE",
		Account:          "user@example.com",
		BundleIdentifier: "app.example",
		CustomIconPath:   uploaded,
	}
	saved, err := SaveApp(first)
	if err != nil {
		t.Fatalf("SaveApp: %v", err)
	}
	want := filepath.Join(dataDir, "ipa", fmt.Sprint(saved.ID), customIconFileName)
	if saved.CustomIconPath != want {
		t.Fatalf("stored custom icon = %q, want %q", saved.CustomIconPath, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("the custom icon was not stored with the record: %v", err)
	}

	// The record keeps the icon for a reinstallation that does not restate it.
	fromDB, err := GetApp(saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fromDB.CustomIconPath != want {
		t.Fatalf("reloaded custom icon = %q, want %q", fromDB.CustomIconPath, want)
	}

	// A source update restates the stored path (ApplyBuild keeps it) and
	// SaveApp keeps the same file. An explicitly empty choice clears it.
	again := *fromDB
	kept, err := SaveApp(again)
	if err != nil {
		t.Fatalf("SaveApp (update): %v", err)
	}
	if kept.CustomIconPath != want {
		t.Fatalf("the custom icon was lost on update: %q", kept.CustomIconPath)
	}

	again.CustomIconPath = ""
	cleared, err := SaveApp(again)
	if err != nil {
		t.Fatalf("SaveApp (clear): %v", err)
	}
	if cleared.CustomIconPath != "" {
		t.Fatalf("an empty choice did not clear the custom icon: %q", cleared.CustomIconPath)
	}
}
