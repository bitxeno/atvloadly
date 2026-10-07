package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/signing"
)

func TestValidateInstallRequestConfinesLocalIPAPathToUploadDir(t *testing.T) {
	dataDir := setTestDataDir(t)
	outsideDir := t.TempDir()

	uploaded := writeTestFile(t, filepath.Join(dataDir, "tmp", "upload.ipa"))
	installed := writeTestFile(t, filepath.Join(dataDir, "ipa", "17", "app.ipa"))
	outside := writeTestFile(t, filepath.Join(outsideDir, "outside.ipa"))
	symlinkInstalled := filepath.Join(dataDir, "tmp", "installed-link.ipa")
	if err := os.Symlink(installed, symlinkInstalled); err != nil {
		t.Fatal(err)
	}
	// The validator resolves the symlink-free path, which differs from the
	// temporary directory path on macOS (/var vs /private/var).
	resolvedUploaded, err := filepath.EvalSymlinks(uploaded)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		mode       model.SigningMode
		path       string
		wantPath   string
		wantReject bool
	}{
		{name: "apple id upload", mode: model.SigningModeAppleID, path: uploaded, wantPath: resolvedUploaded},
		{name: "apple id remote", mode: model.SigningModeAppleID, path: "https://example.com/app.ipa", wantPath: "https://example.com/app.ipa"},
		{name: "apple id installed app", mode: model.SigningModeAppleID, path: installed, wantReject: true},
		{name: "apple id outside data dir", mode: model.SigningModeAppleID, path: outside, wantReject: true},
		{name: "apple id symlink to installed app", mode: model.SigningModeAppleID, path: symlinkInstalled, wantReject: true},
		{name: "apple id empty path", mode: model.SigningModeAppleID, path: "", wantReject: true},
		{name: "external upload", mode: model.SigningModeExternalCertificate, path: uploaded, wantPath: resolvedUploaded},
		{name: "external installed app", mode: model.SigningModeExternalCertificate, path: installed, wantReject: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := model.InstalledApp{
				IpaPath:     tt.path,
				UDID:        "00008120-0000000000000001",
				SigningMode: tt.mode,
			}
			if tt.mode == model.SigningModeExternalCertificate {
				v.SigningIdentityID = 1
			} else {
				v.Account = "apple@example.com"
			}

			err := validateInstallRequest(&v)
			if tt.wantReject {
				if code := signing.CodeOf(err); code != signing.CodeIPAPathRejected {
					t.Fatalf("validateInstallRequest(%q) error = %v (code %q), want code %q", tt.path, err, code, signing.CodeIPAPathRejected)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateInstallRequest(%q): %v", tt.path, err)
			}
			if v.IpaPath != tt.wantPath {
				t.Fatalf("IpaPath = %q, want %q", v.IpaPath, tt.wantPath)
			}
		})
	}
}
