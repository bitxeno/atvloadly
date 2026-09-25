package task

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/bitxeno/atvloadly/internal/ipa"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/robfig/cron/v3"
)

func TestShouldUseRefreshMode(t *testing.T) {
	newApp := model.InstalledApp{}
	if shouldUseRefreshMode(newApp) || isSourceUpdate(newApp) {
		t.Fatal("new app install should neither use refresh mode nor be an update")
	}

	newURLApp := model.InstalledApp{IpaPath: "https://example.com/app.ipa"}
	if shouldUseRefreshMode(newURLApp) || isSourceUpdate(newURLApp) {
		t.Fatal("new app install from a URL should neither use refresh mode nor be an update")
	}

	existingApp := model.InstalledApp{IpaPath: "/data/ipa/1/app.ipa"}
	existingApp.ID = 1
	if !shouldUseRefreshMode(existingApp) || isSourceUpdate(existingApp) {
		t.Fatal("existing app refresh should use refresh mode")
	}

	updatedApp := model.InstalledApp{IpaPath: "https://github.com/owner/repo/releases/download/v2/App.ipa"}
	updatedApp.ID = 1
	if shouldUseRefreshMode(updatedApp) || !isSourceUpdate(updatedApp) {
		t.Fatal("installing a new build of an existing app should sign it again")
	}
}

func TestVerifyIPA(t *testing.T) {
	tvApp := model.InstalledApp{DeviceClass: "AppleTV", BundleIdentifier: "com.example.app"}
	installed := tvApp
	installed.ID = 3

	cases := []struct {
		name    string
		v       model.InstalledApp
		result  ipa.DownloadResult
		wantErr bool
	}{
		{"new install", tvApp, ipa.DownloadResult{BundleIdentifier: "com.other.app", Platforms: []string{"AppleTVOS"}}, false},
		{"unknown platform", tvApp, ipa.DownloadResult{BundleIdentifier: "com.other.app"}, false},
		{"iOS IPA on Apple TV", tvApp, ipa.DownloadResult{Platforms: []string{"iPhoneOS"}}, true},
		{"update", installed, ipa.DownloadResult{BundleIdentifier: "com.example.app", Platforms: []string{"AppleTVOS"}}, false},
		{"update with another bundle", installed, ipa.DownloadResult{BundleIdentifier: "com.example.app.beta", Platforms: []string{"AppleTVOS"}}, true},
	}
	for _, c := range cases {
		err := verifyIPA(c.v, &c.result)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: verifyIPA = %v, wantErr %v", c.name, err, c.wantErr)
		}
	}

	err := verifyIPA(installed, &ipa.DownloadResult{BundleIdentifier: "com.example.app.beta"})
	if err == nil || err.Error() != "bundle identifier changed from com.example.app to com.example.app.beta; install it as a new app" {
		t.Errorf("unexpected error: %v", err)
	}
	if err := verifyIPA(tvApp, &ipa.DownloadResult{Platforms: []string{"iPhoneOS"}}); !errors.Is(err, ipa.ErrPlatformMismatch) {
		t.Errorf("platform mismatch error = %v", err)
	}
}

func TestStartInstallAppsQueuedCount(t *testing.T) {
	tk := new()
	app := func(id uint) model.InstalledApp {
		v := model.InstalledApp{IpaName: "app"}
		v.ID = id
		return v
	}

	if n := tk.StartInstallApps(nil, false); n != 0 {
		t.Fatalf("queued %d apps, want 0", n)
	}
	if n := tk.StartInstallApps([]model.InstalledApp{app(1), app(2), app(1)}, false); n != 2 {
		t.Fatalf("queued %d apps, want 2", n)
	}
	if tk.currentBatch == nil || tk.currentBatch.TotalCount != 2 {
		t.Fatalf("batch = %+v, want 2 queued apps", tk.currentBatch)
	}

	// Apps already installing are not queued again and the batch in flight
	// stays current.
	batch := tk.currentBatch
	if n := tk.StartInstallApps([]model.InstalledApp{app(1), app(2)}, false); n != 0 {
		t.Fatalf("queued %d apps, want 0", n)
	}
	if tk.currentBatch != batch || batch.TotalCount != 2 {
		t.Fatalf("batch = %+v, want the batch in flight %+v", tk.currentBatch, batch)
	}
	if len(tk.InstallAppQueue) != 2 {
		t.Fatalf("%d queued items, want 2", len(tk.InstallAppQueue))
	}
	if !tk.isInstalling(1) || !tk.isInstalling(2) || tk.isInstalling(3) {
		t.Fatal("unexpected isInstalling result")
	}
}

func TestStartInstallAppsKeepsBatchInFlight(t *testing.T) {
	tk := new()
	app := func(id uint) model.InstalledApp {
		v := model.InstalledApp{IpaName: "app"}
		v.ID = id
		return v
	}

	if n := tk.StartInstallApps([]model.InstalledApp{app(1), app(2)}, false); n != 2 {
		t.Fatalf("queued %d apps, want 2", n)
	}
	// An update or a scheduled run while the batch installs queues nothing.
	if n := tk.StartInstallApps([]model.InstalledApp{app(2)}, false); n != 0 {
		t.Fatalf("queued %d apps, want 0", n)
	}

	// The batch in flight still completes once both queued apps are done.
	first, second := <-tk.InstallAppQueue, <-tk.InstallAppQueue
	tk.trackBatchProgress(first, true, nil)
	if tk.currentBatch == nil || tk.currentBatch.ID != first.BatchID || tk.currentBatch.SuccessCount != 1 {
		t.Fatalf("batch after one app = %+v, want batch %s with one success", tk.currentBatch, first.BatchID)
	}
	tk.trackBatchProgress(second, false, errors.New("boom"))
	if tk.currentBatch != nil {
		t.Fatalf("completed batch kept: %+v", tk.currentBatch)
	}
}

func TestUpdateCheckSpec(t *testing.T) {
	cases := map[int]string{
		1:  "17 */1 * * *",
		3:  "17 */3 * * *",
		6:  "17 */6 * * *",
		12: "17 */12 * * *",
		24: "17 0 * * *",
		5:  "17 */6 * * *",
		48: "17 */6 * * *",
	}
	for hours, want := range cases {
		got := updateCheckSpec(hours)
		if got != want {
			t.Errorf("updateCheckSpec(%d) = %q, want %q", hours, got, want)
		}
		if _, err := cron.ParseStandard(got); err != nil {
			t.Errorf("updateCheckSpec(%d) = %q is invalid: %v", hours, got, err)
		}
	}
}

func TestSelectAutoRefreshApps(t *testing.T) {
	now := time.Now()
	refreshed := now.Add(-24 * time.Hour)
	dueSoon := now.Add(12 * time.Hour)
	notDue := now.Add(30 * 24 * time.Hour)

	app := func(id uint, mode model.SigningMode, expiration time.Time, refreshedError model.RefreshedError) model.InstalledApp {
		v := model.InstalledApp{
			Account:          "user@example.com",
			RefreshedDate:    &refreshed,
			ExpirationDate:   &expiration,
			RefreshedError:   refreshedError,
			SigningMode:      mode,
			BundleIdentifier: "com.example.app",
		}
		if mode == model.SigningModeExternalCertificate {
			v.Account = ""
			v.SigningIdentityID = 7
		}
		v.ID = id
		return v
	}

	tests := []struct {
		name         string
		apps         []model.InstalledApp
		wantRefresh  []uint
		wantExternal []uint
	}{
		{
			name:        "apple id apps due for refresh, including historical rows without mode",
			apps:        []model.InstalledApp{app(1, model.SigningModeAppleID, dueSoon, 0), app(2, "", dueSoon, 0)},
			wantRefresh: []uint{1, 2},
		},
		{
			name: "apps not due are ignored",
			apps: []model.InstalledApp{app(1, model.SigningModeAppleID, notDue, 0), app(2, model.SigningModeExternalCertificate, notDue, 0)},
		},
		{
			name: "invalid account is skipped",
			apps: []model.InstalledApp{app(1, model.SigningModeAppleID, dueSoon, model.RefreshedErrorInvalidAccount)},
		},
		{
			name:         "external certificate apps are never refreshed automatically",
			apps:         []model.InstalledApp{app(1, model.SigningModeExternalCertificate, dueSoon, 0), app(2, model.SigningModeAppleID, dueSoon, 0)},
			wantRefresh:  []uint{2},
			wantExternal: []uint{1},
		},
		{
			name:         "expired external app with a failed reinstall is still skipped",
			apps:         []model.InstalledApp{app(1, model.SigningModeExternalCertificate, now.Add(-time.Hour), model.RefreshedErrorTransport)},
			wantExternal: []uint{1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refresh, external := selectAutoRefreshApps(tt.apps, 1)
			if got := appIDs(refresh); !reflect.DeepEqual(got, tt.wantRefresh) {
				t.Fatalf("refresh = %v, want %v", got, tt.wantRefresh)
			}
			if got := appIDs(external); !reflect.DeepEqual(got, tt.wantExternal) {
				t.Fatalf("external = %v, want %v", got, tt.wantExternal)
			}
		})
	}
}

func appIDs(apps []model.InstalledApp) []uint {
	var ids []uint
	for _, v := range apps {
		ids = append(ids, v.ID)
	}
	return ids
}
