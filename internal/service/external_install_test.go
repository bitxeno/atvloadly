package service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	conf "github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/manager"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/signing"
	"github.com/bitxeno/atvloadly/internal/signing/appcheck"
)

// setTestDataDir points the application data directory to a fresh temporary
// directory with its upload (tmp) and installed apps (ipa) directories.
func setTestDataDir(t *testing.T) string {
	t.Helper()
	previous := conf.Config
	dataDir := t.TempDir()
	conf.Config = &conf.Configuration{}
	conf.Config.Server.DataDir = dataDir
	t.Cleanup(func() { conf.Config = previous })
	for _, dir := range []string{"tmp", "ipa"} {
		if err := os.MkdirAll(filepath.Join(dataDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dataDir
}

func writeTestFile(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("ipa"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveClientIPAPath(t *testing.T) {
	dataDir := setTestDataDir(t)
	outside := t.TempDir()

	uploaded := writeTestFile(t, filepath.Join(dataDir, "tmp", "app_1.ipa"))
	installed := writeTestFile(t, filepath.Join(dataDir, "ipa", "3", "app.ipa"))
	secret := writeTestFile(t, filepath.Join(outside, "secret.ipa"))
	dataFile := writeTestFile(t, filepath.Join(dataDir, "atvloadly.db"))

	symlinkInside := filepath.Join(dataDir, "tmp", "link_inside.ipa")
	if err := os.Symlink(installed, symlinkInside); err != nil {
		t.Fatal(err)
	}
	symlinkEscape := filepath.Join(dataDir, "tmp", "link_escape.ipa")
	if err := os.Symlink(secret, symlinkEscape); err != nil {
		t.Fatal(err)
	}
	dirEscape := filepath.Join(dataDir, "ipa", "escape")
	if err := os.Symlink(outside, dirEscape); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "uploaded file", path: uploaded, want: uploaded},
		{name: "installed app file", path: installed, want: installed},
		{name: "symlink to an installed app", path: symlinkInside, want: installed},
		{name: "relative traversal is cleaned inside", path: filepath.Join(dataDir, "ipa", "3", "..", "3", "app.ipa"), want: installed},
		{name: "empty path", path: ""},
		{name: "outside the data directory", path: secret},
		{name: "data directory file outside tmp and ipa", path: dataFile},
		{name: "traversal out of tmp", path: filepath.Join(dataDir, "tmp", "..", "atvloadly.db")},
		{name: "traversal out of the data directory", path: filepath.Join(dataDir, "ipa", "..", "..", filepath.Base(outside), "secret.ipa")},
		{name: "symlinked file escaping tmp", path: symlinkEscape},
		{name: "symlinked directory escaping ipa", path: filepath.Join(dirEscape, "secret.ipa")},
		{name: "allowed directory itself", path: filepath.Join(dataDir, "tmp")},
		{name: "missing file", path: filepath.Join(dataDir, "tmp", "missing.ipa")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveClientIPAPath(tt.path)
			if tt.want == "" {
				if code := signing.CodeOf(err); code != signing.CodeIPAPathRejected {
					t.Fatalf("ResolveClientIPAPath(%q) = %q, %v; want code %q", tt.path, got, err, signing.CodeIPAPathRejected)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveClientIPAPath(%q): %v", tt.path, err)
			}
			want, _ := filepath.EvalSymlinks(tt.want)
			if got != want {
				t.Fatalf("ResolveClientIPAPath(%q) = %q, want %q", tt.path, got, want)
			}
		})
	}
}

// A refused preparation must never keep the identity leased: a leased
// identity cannot be deleted nor get its profile replaced.
func TestPrepareExternalInstallFailuresReleaseLease(t *testing.T) {
	openTestStore(t)
	dataDir := setTestDataDir(t)
	identity := mustCreateIdentity(t, testIdentity("cert-a", "profile-a"))
	ipaPath := writeTestFile(t, filepath.Join(dataDir, "tmp", "app_1.ipa"))
	outsidePath := writeTestFile(t, filepath.Join(t.TempDir(), "app.ipa"))
	unreachable := func(*model.Device) (*model.DeviceInfo, error) {
		return nil, errors.New("device socket io failed")
	}

	tests := []struct {
		name       string
		identityID uint
		ipaPath    string
		wantClass  signing.Class
		wantCode   string
	}{
		{name: "missing identity", identityID: identity.ID + 100, ipaPath: ipaPath, wantClass: signing.ClassIdentity, wantCode: signing.CodeIdentityNotFound},
		{name: "rejected ipa path", identityID: identity.ID, ipaPath: outsidePath, wantClass: signing.ClassSigning, wantCode: signing.CodeIPAPathRejected},
		{name: "unreachable device", identityID: identity.ID, ipaPath: ipaPath, wantClass: signing.ClassTransport, wantCode: signing.CodeDeviceUnreachable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := model.InstalledApp{
				UDID:              "00008120-0000000000000001",
				IpaPath:           tt.ipaPath,
				SigningMode:       model.SigningModeExternalCertificate,
				SigningIdentityID: tt.identityID,
			}
			dev := &model.Device{UDID: app.UDID, DeviceClass: string(model.DeviceClassiPhone)}
			var reports []string
			ext, err := prepareExternalInstall(app, dev, func(line string) { reports = append(reports, line) }, unreachable)
			if ext != nil {
				ext.Close()
				t.Fatal("preparation succeeded, want an error")
			}
			if signing.ClassOf(err) != tt.wantClass || signing.CodeOf(err) != tt.wantCode {
				t.Fatalf("err = %v (class %q code %q), want class %q code %q", err, signing.ClassOf(err), signing.CodeOf(err), tt.wantClass, tt.wantCode)
			}
			if signing.LeaseHeld(tt.identityID) {
				t.Fatal("identity still leased after a failed preparation")
			}
			if len(reports) != 0 {
				t.Fatalf("plan reported before the plan was built: %q", reports)
			}
		})
	}
}

func TestRefreshedErrorOf(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want model.RefreshedError
	}{
		{name: "invalid Apple account", err: manager.ErrAccountInvalid, want: model.RefreshedErrorInvalidAccount},
		{name: "identity", err: signing.Errorf(signing.ClassIdentity, signing.CodeIncompatible, "x"), want: model.RefreshedErrorSigningIdentity},
		{name: "signing", err: signing.Errorf(signing.ClassSigning, signing.CodeEngineFailed, "x"), want: model.RefreshedErrorSigning},
		{name: "transport", err: signing.Errorf(signing.ClassTransport, signing.CodeTransportFailed, "x"), want: model.RefreshedErrorTransport},
		{name: "other", err: errors.New("x"), want: model.RefreshedErrorInvalidOther},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RefreshedErrorOf(tt.err); got != tt.want {
				t.Fatalf("RefreshedErrorOf(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestSaveAppKeepsSigningModesApart(t *testing.T) {
	const udid, bundleID, account = "DEVICE", "com.example.app", "user@example.com"
	appleID := func(mode model.SigningMode) model.InstalledApp {
		return model.InstalledApp{UDID: udid, BundleIdentifier: bundleID, Account: account, SigningMode: mode}
	}
	external := func(identityID uint) model.InstalledApp {
		return model.InstalledApp{UDID: udid, BundleIdentifier: bundleID, SigningMode: model.SigningModeExternalCertificate, SigningIdentityID: identityID, SignedBundleIdentifier: bundleID}
	}
	customized := func(app model.InstalledApp, bundleID, customIdentifier string) model.InstalledApp {
		app.BundleIdentifier = bundleID
		app.CustomIdentifier = customIdentifier
		app.SignedBundleIdentifier = customIdentifier
		return app
	}

	tests := []struct {
		name      string
		existing  model.InstalledApp
		saved     model.InstalledApp
		wantMerge bool
	}{
		{name: "apple id reinstall", existing: appleID(model.SigningModeAppleID), saved: appleID(model.SigningModeAppleID), wantMerge: true},
		{name: "apple id reinstall of a historical row", existing: appleID(""), saved: appleID(""), wantMerge: true},
		{name: "external reinstall with the same identity", existing: external(1), saved: external(1), wantMerge: true},
		{name: "external install over an apple id row", existing: appleID(""), saved: external(1)},
		{name: "apple id install over an external row", existing: external(1), saved: appleID(model.SigningModeAppleID)},
		{name: "external install with another identity", existing: external(1), saved: external(2)},
		{name: "external reinstall under the same custom identifier", existing: customized(external(1), bundleID, "app.custom"), saved: customized(external(1), bundleID, "app.custom"), wantMerge: true},
		{name: "external install of the same app under a custom identifier", existing: external(1), saved: customized(external(1), bundleID, "app.custom")},
		{name: "external install of another app under the same identifier", existing: customized(external(1), bundleID, "app.custom"), saved: customized(external(1), "com.example.other", "app.custom"), wantMerge: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openTestStore(t)
			setTestDataDir(t)
			existing, err := SaveApp(tt.existing)
			if err != nil {
				t.Fatalf("save existing: %v", err)
			}
			saved, err := SaveApp(tt.saved)
			if err != nil {
				t.Fatalf("save: %v", err)
			}
			if merged := saved.ID == existing.ID; merged != tt.wantMerge {
				t.Fatalf("merged = %v, want %v", merged, tt.wantMerge)
			}

			stored, err := GetApp(saved.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.EffectiveSigningMode() != tt.saved.EffectiveSigningMode() || stored.SigningIdentityID != tt.saved.SigningIdentityID {
				t.Fatalf("stored mode %q identity %d, want %q identity %d", stored.SigningMode, stored.SigningIdentityID, tt.saved.EffectiveSigningMode(), tt.saved.SigningIdentityID)
			}
			if stored.Account != tt.saved.Account || stored.SignedBundleIdentifier != tt.saved.SignedBundleIdentifier {
				t.Fatalf("stored account %q signed id %q, want %q %q", stored.Account, stored.SignedBundleIdentifier, tt.saved.Account, tt.saved.SignedBundleIdentifier)
			}
			if stored.BundleIdentifier != tt.saved.BundleIdentifier || stored.CustomIdentifier != tt.saved.CustomIdentifier {
				t.Fatalf("stored bundle id %q custom id %q, want %q %q", stored.BundleIdentifier, stored.CustomIdentifier, tt.saved.BundleIdentifier, tt.saved.CustomIdentifier)
			}
		})
	}
}

// A failed external reinstallation is recorded on the record of the app it
// would have replaced on the device, found from its custom identifier.
func TestRecordExternalInstallFailureFindsTheAppOnTheDevice(t *testing.T) {
	openTestStore(t)
	setTestDataDir(t)
	installed, err := SaveApp(model.InstalledApp{
		UDID: "DEVICE", BundleIdentifier: "com.example.app", SigningMode: model.SigningModeExternalCertificate, SigningIdentityID: 1,
		CustomIdentifier: "app.custom", SignedBundleIdentifier: "app.custom", RefreshedResult: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	failure := signing.Errorf(signing.ClassTransport, signing.CodeTransportFailed, "device lost")
	request := model.InstalledApp{UDID: "DEVICE", BundleIdentifier: "com.example.app", SigningMode: model.SigningModeExternalCertificate, SigningIdentityID: 1}

	// Without the custom identifier the request targets another app.
	recordExternalInstallFailure(request, failure)
	if stored, err := GetApp(installed.ID); err != nil || !stored.RefreshedResult {
		t.Fatalf("record of another app marked failed: %+v, %v", stored, err)
	}

	request.CustomIdentifier = "app.custom"
	recordExternalInstallFailure(request, failure)
	stored, err := GetApp(installed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.RefreshedResult || stored.RefreshedError != model.RefreshedErrorTransport {
		t.Fatalf("stored result %v error %d, want a transport failure", stored.RefreshedResult, stored.RefreshedError)
	}
}

// The icon path of an installation comes from the client: SaveApp must never
// move nor record a file outside the upload and installed apps directories.
func TestSaveAppMovesOnlyUploadedIcons(t *testing.T) {
	openTestStore(t)
	dataDir := setTestDataDir(t)
	outside := writeTestFile(t, filepath.Join(t.TempDir(), "secret.png"))
	link := filepath.Join(dataDir, "tmp", "link.png")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	withIcon := func(icon string) model.InstalledApp {
		return model.InstalledApp{UDID: "DEVICE", BundleIdentifier: "com.example.app", Account: "user@example.com", Icon: icon}
	}
	storedIcon := func(id uint) string {
		t.Helper()
		stored, err := GetApp(id)
		if err != nil {
			t.Fatal(err)
		}
		return stored.Icon
	}
	assertOutsideKept := func() {
		t.Helper()
		if _, err := os.Stat(outside); err != nil {
			t.Fatalf("the icon outside the data directories was moved: %v", err)
		}
	}

	saved, err := SaveApp(withIcon(outside))
	if err != nil {
		t.Fatal(err)
	}
	assertOutsideKept()
	if icon := storedIcon(saved.ID); icon != "" {
		t.Fatalf("new record icon = %q, want none", icon)
	}

	uploaded := writeTestFile(t, filepath.Join(dataDir, "tmp", "app_1.png"))
	if _, err := SaveApp(withIcon(uploaded)); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dataDir, "ipa", strconv.FormatUint(uint64(saved.ID), 10), "app.png")
	if icon := storedIcon(saved.ID); icon != want {
		t.Fatalf("icon = %q, want the uploaded icon moved to %q", icon, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("uploaded icon not moved: %v", err)
	}

	for _, icon := range []string{outside, link} {
		if _, err := SaveApp(withIcon(icon)); err != nil {
			t.Fatal(err)
		}
		assertOutsideKept()
		if got := storedIcon(saved.ID); got != want {
			t.Fatalf("icon after a reinstall with %q = %q, want the previous %q", icon, got, want)
		}
	}
}

// An external installation cleans up its own upload files only: other
// uploads sharing their name and the files of installed apps stay.
func TestCleanExternalUploadRemovesOnlyItsUploadFiles(t *testing.T) {
	dataDir := setTestDataDir(t)
	ipaPath := writeTestFile(t, filepath.Join(dataDir, "tmp", "app_1.ipa"))
	iconPath := writeTestFile(t, filepath.Join(dataDir, "tmp", "app_1.png"))
	sibling := writeTestFile(t, filepath.Join(dataDir, "tmp", "app_1_other.ipa"))
	storedIPA := writeTestFile(t, filepath.Join(dataDir, "ipa", "1", "app.ipa"))
	storedIcon := writeTestFile(t, filepath.Join(dataDir, "ipa", "1", "app.png"))
	outside := writeTestFile(t, filepath.Join(t.TempDir(), "app_1.ipa"))

	CleanExternalUpload(ipaPath, iconPath)
	CleanExternalUpload(storedIPA, storedIcon)
	CleanExternalUpload(outside, "")

	for _, removed := range []string{ipaPath, iconPath} {
		if _, err := os.Stat(removed); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s not removed: %v", removed, err)
		}
	}
	for _, kept := range []string{sibling, storedIPA, storedIcon, outside} {
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("%s removed: %v", kept, err)
		}
	}
}

// Apple ID requests keep their IPA path as given; external certificate
// requests are confined to the upload and installed apps directories, only
// they accept a custom bundle identifier, and they cannot track a source.
func TestValidateInstallRequest(t *testing.T) {
	dataDir := setTestDataDir(t)
	uploaded := writeTestFile(t, filepath.Join(dataDir, "tmp", "app_1.ipa"))
	outside := writeTestFile(t, filepath.Join(t.TempDir(), "app.ipa"))

	tests := []struct {
		name     string
		request  model.InstalledApp
		wantErr  bool
		wantPath string
		wantID   string
	}{
		{name: "apple id path passed through", request: model.InstalledApp{UDID: "DEVICE", Account: "user@example.com", IpaPath: outside}, wantPath: outside},
		{name: "apple id custom identifier", request: model.InstalledApp{UDID: "DEVICE", Account: "user@example.com", IpaPath: uploaded, CustomIdentifier: "app.custom"}, wantErr: true},
		{name: "external path outside the data directories", request: model.InstalledApp{UDID: "DEVICE", IpaPath: outside, SigningMode: model.SigningModeExternalCertificate, SigningIdentityID: 1}, wantErr: true},
		{name: "external custom identifier trimmed", request: model.InstalledApp{UDID: "DEVICE", IpaPath: uploaded, SigningMode: model.SigningModeExternalCertificate, SigningIdentityID: 1, CustomIdentifier: " app.custom "}, wantPath: uploaded, wantID: "app.custom"},
		{name: "external tracked source", request: model.InstalledApp{UDID: "DEVICE", IpaPath: uploaded, SigningMode: model.SigningModeExternalCertificate, SigningIdentityID: 1, Source: model.AppSource{Kind: "github", URL: "owner/repo", BuildID: "1"}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := tt.request
			err := validateInstallRequest(&request)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, want error %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if request.IpaPath != tt.wantPath || request.CustomIdentifier != tt.wantID {
				t.Fatalf("ipa path %q custom id %q, want %q %q", request.IpaPath, request.CustomIdentifier, tt.wantPath, tt.wantID)
			}
		})
	}
}

// The streamed plan names the application identifier of the main app, which
// the install page shows next to the signed bundle identifier.
func TestPlanReportLineMainApplicationIdentifier(t *testing.T) {
	plan := &appcheck.Plan{
		Issues:             []signing.Issue{},
		MainBundleID:       "com.example.app",
		SignedMainBundleID: "com.example.app",
		Bundles: []appcheck.PlannedBundle{
			{Path: "Payload/App.app/PlugIns/Widget.appex", OriginalID: "com.example.app.widget", SignedID: "com.example.app.widget", ExpectedApplicationIdentifier: "ABCDE12345.com.example.widget"},
			{Path: "Payload/App.app", OriginalID: "com.example.app", SignedID: "com.example.app", ExpectedApplicationIdentifier: "ABCDE12345.com.example.profile"},
		},
	}
	line := planReportLine(plan)
	var report struct {
		MainApplicationIdentifier string `json:"main_application_identifier"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(line, SigningReportPrefix)), &report); err != nil {
		t.Fatalf("decode %q: %v", line, err)
	}
	if report.MainApplicationIdentifier != "ABCDE12345.com.example.profile" {
		t.Fatalf("main_application_identifier = %q, want the main app one", report.MainApplicationIdentifier)
	}
}
