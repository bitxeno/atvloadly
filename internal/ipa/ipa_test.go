package ipa

import (
	"archive/zip"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseFile(t *testing.T) {
	ipaPath := filepath.Join(t.TempDir(), "fixture.ipa")
	writeFixtureIPA(t, ipaPath)

	parsed, err := ParseFile(ipaPath)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	if parsed.Name() != "Fixture TV" {
		t.Fatalf("Name() = %q, want %q", parsed.Name(), "Fixture TV")
	}
	if parsed.Identifier() != "com.example.fixture" {
		t.Fatalf("Identifier() = %q, want %q", parsed.Identifier(), "com.example.fixture")
	}
	if parsed.Version() != "1.2.3" {
		t.Fatalf("Version() = %q, want %q", parsed.Version(), "1.2.3")
	}
	if got := parsed.Platforms(); !reflect.DeepEqual(got, []string{"AppleTVOS"}) {
		t.Fatalf("Platforms() = %v, want [AppleTVOS]", got)
	}

	icon := parsed.Icon()
	if icon == nil {
		t.Fatal("Icon() returned nil")
	}
	if got := icon.Bounds(); got.Dx() != 120 || got.Dy() != 120 {
		t.Fatalf("icon bounds = %v, want 120x120", got)
	}
}

func TestParseFileReturnsErrorForMissingFile(t *testing.T) {
	_, err := ParseFile(filepath.Join(t.TempDir(), "missing.ipa"))
	if err == nil {
		t.Fatal("ParseFile returned nil error for a missing file")
	}
}

func writeFixtureIPA(t *testing.T, ipaPath string) {
	t.Helper()

	file, err := os.Create(ipaPath)
	if err != nil {
		t.Fatalf("create fixture IPA: %v", err)
	}

	archive := zip.NewWriter(file)
	plistFile, err := archive.Create("Payload/Fixture.app/Info.plist")
	if err != nil {
		t.Fatalf("create Info.plist: %v", err)
	}
	if _, err := io.WriteString(plistFile, fixtureInfoPlist); err != nil {
		t.Fatalf("write Info.plist: %v", err)
	}

	iconFile, err := archive.Create("Payload/Fixture.app/AppIcon60x60@2x.png")
	if err != nil {
		t.Fatalf("create icon: %v", err)
	}
	if err := png.Encode(iconFile, image.NewRGBA(image.Rect(0, 0, 120, 120))); err != nil {
		t.Fatalf("write icon: %v", err)
	}

	if err := archive.Close(); err != nil {
		t.Fatalf("close IPA archive: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close IPA file: %v", err)
	}
}

const fixtureInfoPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>CFBundleDisplayName</key><string>Fixture TV</string>
  <key>CFBundleIdentifier</key><string>com.example.fixture</string>
  <key>CFBundleShortVersionString</key><string>1.2.3</string>
  <key>CFBundleVersion</key><string>123</string>
  <key>CFBundleSupportedPlatforms</key><array><string>AppleTVOS</string></array>
</dict></plist>`
