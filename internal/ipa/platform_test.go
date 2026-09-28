package ipa

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bitxeno/atvloadly/internal/app"
)

func TestCheckPlatform(t *testing.T) {
	cases := []struct {
		platforms   []string
		deviceClass string
		wantErr     bool
	}{
		{[]string{"AppleTVOS"}, "AppleTV", false},
		{[]string{"iPhoneOS"}, "iPhone", false},
		{[]string{"iPhoneOS"}, "iPad", false},
		{[]string{"iphoneos"}, "iPad", false},
		{[]string{"iPhoneOS"}, "AppleTV", true},
		{[]string{"AppleTVOS"}, "iPhone", true},
		{[]string{"AppleTVSimulator"}, "AppleTV", true},
		{[]string{"iPhoneOS", "AppleTVOS"}, "AppleTV", false},
		{nil, "AppleTV", false},
		{[]string{"iPhoneOS"}, "", false},
		{[]string{"iPhoneOS"}, "Watch", false},
	}
	for _, c := range cases {
		err := CheckPlatform(c.platforms, c.deviceClass)
		if (err != nil) != c.wantErr {
			t.Errorf("CheckPlatform(%v, %q) = %v, wantErr %v", c.platforms, c.deviceClass, err, c.wantErr)
		}
		if err != nil && !errors.Is(err, ErrPlatformMismatch) {
			t.Errorf("CheckPlatform(%v, %q) = %v, want ErrPlatformMismatch", c.platforms, c.deviceClass, err)
		}
	}

	err := CheckPlatform([]string{"iPhoneOS"}, "AppleTV")
	if err == nil || err.Error() != "ipa platform does not match the device: IPA is built for iPhoneOS, device is AppleTV" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestIsRemoteURL(t *testing.T) {
	cases := map[string]bool{
		"https://example.com/app.ipa": true,
		"http://example.com/app.ipa":  true,
		"HTTPS://example.com/app.ipa": true,
		"/data/ipa/1/app.ipa":         false,
		"httpdocs/app.ipa":            false,
		"":                            false,
	}
	for p, want := range cases {
		if got := IsRemoteURL(p); got != want {
			t.Errorf("IsRemoteURL(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestDownloadResultPlatforms(t *testing.T) {
	dataDir := t.TempDir()
	old := app.Config
	app.Config = &app.Configuration{}
	app.Config.Server.DataDir = dataDir
	t.Cleanup(func() { app.Config = old })

	ipaPath := filepath.Join(t.TempDir(), "fixture.ipa")
	writeFixtureIPA(t, ipaPath)

	local, err := ParseLocalIPA(ipaPath)
	if err != nil {
		t.Fatalf("ParseLocalIPA: %v", err)
	}
	if !reflect.DeepEqual(local.Platforms, []string{"AppleTVOS"}) {
		t.Errorf("ParseLocalIPA Platforms = %v, want [AppleTVOS]", local.Platforms)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, ipaPath)
	}))
	defer srv.Close()

	downloaded, err := DownloadAndParse(srv.URL+"/fixture.ipa", nil)
	if err != nil {
		t.Fatalf("DownloadAndParse: %v", err)
	}
	defer func() { _ = os.Remove(downloaded.LocalPath) }()
	if !reflect.DeepEqual(downloaded.Platforms, []string{"AppleTVOS"}) || downloaded.BundleIdentifier != "com.example.fixture" {
		t.Errorf("DownloadAndParse = %+v", downloaded)
	}
}
