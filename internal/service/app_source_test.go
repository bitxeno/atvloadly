package service

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/db"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/source"
)

// setupTestDB opens an empty database and data directory for one test.
func setupTestDB(t *testing.T) {
	t.Helper()
	oldConfig, oldSettings := app.Config, app.Settings
	app.Config = &app.Configuration{}
	app.Config.Server.DataDir = t.TempDir()
	app.Settings = &app.SettingsConfiguration{}
	app.Settings.Task.Enabled = true
	t.Cleanup(func() { app.Config, app.Settings = oldConfig, oldSettings })

	if err := db.Open(db.Config{Path: t.TempDir(), FileName: "test.db"}).AutoMigrate(&model.InstalledApp{}, &model.SavedSource{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	sqlDB, err := db.Store().DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
}

// serveSource serves an AltStore source and counts the requests it receives.
func serveSource(t *testing.T, body string) (string, *int32) {
	t.Helper()
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/apps.json", &count
}

const testSource = `{"name":"Test source","apps":[
	{"name":"Alpha","bundleIdentifier":"com.example.alpha","subtitle":"for tvOS","versions":[
		{"version":"2.0","date":"2026-09-20T10:00:00Z","downloadURL":"https://example.com/dl/Alpha-2.0.ipa","size":2000},
		{"version":"1.0","date":"2026-09-01T10:00:00Z","downloadURL":"https://example.com/dl/Alpha-1.0.ipa","size":1000}
	]},
	{"name":"Beta","bundleIdentifier":"com.example.beta","versions":[
		{"version":"5.0","date":"2026-09-21T10:00:00Z","downloadURL":"https://example.com/dl/Beta-5.0.ipa","size":5000}
	]}
]}`

// latestBuild returns the newest version of a test source app.
func latestBuild(t *testing.T, u, bundleID string) source.Build {
	t.Helper()
	feed, err := source.Fetch(source.KindAltStore, u)
	if err != nil {
		t.Fatal(err)
	}
	b, err := feed.Latest(bundleID, false, "")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func createApp(t *testing.T, v model.InstalledApp) model.InstalledApp {
	t.Helper()
	saved, err := SaveApp(v)
	if err != nil {
		t.Fatalf("SaveApp: %v", err)
	}
	return *saved
}

func mustGetApp(t *testing.T, id uint) model.InstalledApp {
	t.Helper()
	v, err := GetApp(id)
	if err != nil {
		t.Fatalf("GetApp(%d): %v", id, err)
	}
	return *v
}

func timePtr(t time.Time) *time.Time { return &t }

func TestEvaluateSource(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	installedDate := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	installed := model.AppSource{
		Kind: source.KindGitHub, URL: "owner/repo", Filter: "tvos",
		BuildID: "100", Version: "v1", BuildDate: &installedDate,
	}
	withLatest := installed
	withLatest.LatestBuildID, withLatest.LatestVersion, withLatest.CheckError = "200", "v2", "old error"
	latestOnly := withLatest
	latestOnly.CheckError = ""
	notLinked := model.AppSource{Kind: source.KindGitHub, URL: "owner/repo", Filter: "tvos"}
	// AltStore dates are often date-only (midnight UTC).
	altStore := model.AppSource{
		Kind: source.KindAltStore, URL: "https://example.com/apps.json", Filter: "com.example.app",
		BuildID: "a120", Version: "1.2.0", BuildDate: &installedDate,
	}
	altStoreStamped := altStore
	altStoreStamped.Version, altStoreStamped.BuildDate = "1.0.0 (135)", timePtr(time.Date(2026, 9, 24, 3, 20, 31, 0, time.UTC))
	sameDay := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name                            string
		s                               model.AppSource
		b                               source.Build
		err                             error
		wantLatest, wantVersion, wantEr string
	}{
		{"network error keeps the last result", latestOnly, source.Build{}, errors.New("timeout"), "200", "v2", "timeout"},
		{"network error keeps an earlier error", withLatest, source.Build{}, errors.New("timeout"), "200", "v2", "old error"},
		{"rate limit keeps the last result", latestOnly, source.Build{}, &source.RateLimitError{Reset: now}, "200", "v2", (&source.RateLimitError{Reset: now}).Error()},
		{"no match clears", withLatest, source.Build{}, fmt.Errorf("%w: renamed", source.ErrNoMatch), "", "", "no matching build: renamed"},
		{"ambiguous clears", withLatest, source.Build{}, fmt.Errorf("%w: choose", source.ErrAmbiguous), "", "", "several builds match: choose"},
		{"installed build", withLatest, source.Build{ID: "100", Version: "v1", Date: installedDate}, nil, "", "", ""},
		{"newer build", installed, source.Build{ID: "200", Version: "v2", Date: installedDate.Add(time.Hour)}, nil, "200", "v2", ""},
		{"re-uploaded asset", installed, source.Build{ID: "101", Version: "v1", Date: installedDate.Add(time.Minute)}, nil, "101", "v1", ""},
		{"older build is no downgrade", withLatest, source.Build{ID: "90", Version: "v0.9", Date: installedDate.Add(-time.Hour)}, nil, "", "", ""},
		{"same date is no update", installed, source.Build{ID: "99", Version: "v1", Date: installedDate}, nil, "", "", ""},
		{"unknown date", installed, source.Build{ID: "200", Version: "v2"}, nil, "200", "v2", ""},
		{"unknown installed build", notLinked, source.Build{ID: "100", Version: "v1", Date: installedDate}, nil, "100", "v1", ""},
		{"same-day AltStore update", altStore, source.Build{ID: "a121", Version: "1.2.1", Date: installedDate}, nil, "a121", "1.2.1", ""},
		{"date-only AltStore update after a timestamped build", altStoreStamped, source.Build{ID: "a101", Version: "1.0.1", Date: sameDay}, nil, "a101", "1.0.1", ""},
		{"moved AltStore download is no update", altStore, source.Build{ID: "a120b", Version: "1.2.0", Date: installedDate}, nil, "", "", ""},
		{"older AltStore build is no downgrade", altStoreStamped, source.Build{ID: "a099", Version: "0.9.9", Date: sameDay.Add(-time.Second)}, nil, "", "", ""},
	}
	for _, c := range cases {
		got := EvaluateSource(c.s, c.b, c.err, now)
		if got.CheckedAt == nil || !got.CheckedAt.Equal(now) {
			t.Errorf("%s: CheckedAt = %v", c.name, got.CheckedAt)
		}
		if got.LatestBuildID != c.wantLatest || got.LatestVersion != c.wantVersion || got.CheckError != c.wantEr {
			t.Errorf("%s: got (%q, %q, %q), want (%q, %q, %q)", c.name,
				got.LatestBuildID, got.LatestVersion, got.CheckError, c.wantLatest, c.wantVersion, c.wantEr)
		}
		if got.BuildID != c.s.BuildID || got.Kind != c.s.Kind || got.Filter != c.s.Filter {
			t.Errorf("%s: link or installed build changed: %+v", c.name, got)
		}
	}
}

func TestApplyBuild(t *testing.T) {
	checked := time.Now()
	v := model.InstalledApp{IpaPath: "/data/ipa/1/app.ipa", IpaName: "Orivio"}
	v.ID = 7
	v.Source = model.AppSource{
		Kind: source.KindGitHub, URL: "owner/repo", Filter: "sideload", AutoUpdate: true,
		BuildID: "1", Version: "v0.07", BuildName: "old.ipa",
		CheckedAt: &checked, CheckError: "update failed: x", LatestBuildID: "301", LatestVersion: "v0.10", FailedBuildID: "301",
	}
	date := time.Date(2026, 9, 21, 17, 13, 0, 0, time.UTC)
	b := source.Build{ID: "301", Name: "OrivioTV-V9.Sideload.ipa", Version: "v0.10", Date: date,
		DownloadURL: "https://github.com/owner/repo/releases/download/v0.10/OrivioTV-V9.Sideload.ipa"}

	got := ApplyBuild(v, b)
	want := v.Source
	want.BuildID, want.Version, want.BuildName, want.BuildDate = "301", "v0.10", "OrivioTV-V9.Sideload.ipa", &date
	want.CheckError, want.LatestBuildID, want.LatestVersion, want.FailedBuildID = "", "", "", ""
	if got.ID != 7 || got.IpaName != "Orivio" || got.IpaPath != b.DownloadURL || !reflect.DeepEqual(got.Source, want) {
		t.Fatalf("ApplyBuild = %+v, want source %+v", got, want)
	}
	if v.Source.BuildID != "1" {
		t.Fatal("ApplyBuild modified its input")
	}

	v.Source.Kind = source.KindAltStore
	got = ApplyBuild(v, source.Build{ID: "abc", Name: "NuvioTVOS", Version: "3.3.7",
		DownloadURL: "https://github.com/bobsupra/NuvioTVOS/releases/download/tvos-beta-3.3.7/NuvioTV-3.3.7-unsigned-release.ipa"})
	if got.Source.BuildName != "NuvioTV-3.3.7-unsigned-release.ipa" || got.Source.BuildDate != nil || got.Source.Version != "3.3.7" {
		t.Fatalf("ApplyBuild(AltStore) = %+v", got.Source)
	}
}

func fullSource() model.AppSource {
	d := func(day int) *time.Time { return timePtr(time.Date(2026, 9, day, 10, 0, 0, 0, time.UTC)) }
	return model.AppSource{
		Kind: source.KindGitHub, URL: "owner/repo", Filter: "(?i)tvos", Prerelease: true, AutoUpdate: true,
		BuildID: "301", Version: "v0.10", BuildName: "App-tvOS.ipa", BuildDate: d(20),
		CheckedAt: d(22), CheckError: "boom", LatestBuildID: "302", LatestVersion: "v0.11", FailedBuildID: "303",
	}
}

func assertSource(t *testing.T, got, want model.AppSource) {
	t.Helper()
	sameTime := func(a, b *time.Time) bool { return (a == nil) == (b == nil) && (a == nil || a.Equal(*b)) }
	if !sameTime(got.BuildDate, want.BuildDate) || !sameTime(got.CheckedAt, want.CheckedAt) {
		t.Fatalf("source dates = %v, %v; want %v, %v", got.BuildDate, got.CheckedAt, want.BuildDate, want.CheckedAt)
	}
	got.BuildDate, got.CheckedAt, want.BuildDate, want.CheckedAt = nil, nil, nil, nil
	if got != want {
		t.Fatalf("source = %+v, want %+v", got, want)
	}
}

func TestSourceColumnsCoverEveryField(t *testing.T) {
	if n := reflect.TypeOf(model.AppSource{}).NumField(); len(sourceColumns(model.AppSource{})) != n {
		t.Fatalf("sourceColumns has %d columns, AppSource has %d fields", len(sourceColumns(model.AppSource{})), n)
	}
}

func TestSaveAppSourceRoundTrip(t *testing.T) {
	setupTestDB(t)

	// A new install from a source stores every source field.
	full := fullSource()
	created := createApp(t, model.InstalledApp{IpaName: "App", UDID: "udid", Account: "a@b.c", BundleIdentifier: "com.example.app", Source: full})
	assertSource(t, mustGetApp(t, created.ID).Source, full)

	// Reinstalling from the source replaces the link and the IPA name.
	next := model.AppSource{Kind: source.KindAltStore, URL: "https://example.com/apps.json", Filter: "com.example.app", BuildID: "abc"}
	updated := createApp(t, model.InstalledApp{IpaName: "App 2", UDID: "udid", Account: "a@b.c", BundleIdentifier: "com.example.app", Source: next})
	if updated.ID != created.ID {
		t.Fatalf("SaveApp created a new row %d, want %d", updated.ID, created.ID)
	}
	got := mustGetApp(t, created.ID)
	if got.IpaName != "App 2" {
		t.Fatalf("IpaName = %q, want %q", got.IpaName, "App 2")
	}
	assertSource(t, got.Source, next)

	// Writing all columns round-trips every field.
	if err := db.Store().Model(&got).Updates(sourceColumns(full)).Error; err != nil {
		t.Fatal(err)
	}
	assertSource(t, mustGetApp(t, created.ID).Source, full)

	// Reinstalling from a file or URL stops tracking.
	createApp(t, model.InstalledApp{IpaName: "App", UDID: "udid", Account: "a@b.c", BundleIdentifier: "com.example.app"})
	assertSource(t, mustGetApp(t, created.ID).Source, model.AppSource{})

	tracked, err := GetTrackedAppList()
	if err != nil || len(tracked) != 0 {
		t.Fatalf("GetTrackedAppList = %d apps, %v; want none", len(tracked), err)
	}
}

// trackedApp returns an app installed from the test source u.
func trackedApp(udid, bundleID, u string, s model.AppSource) model.InstalledApp {
	s.Kind, s.URL, s.Filter = source.KindAltStore, u, bundleID
	return model.InstalledApp{IpaName: bundleID, UDID: udid, Account: "a@b.c", BundleIdentifier: bundleID, DeviceClass: "AppleTV", Source: s}
}

func TestCheckSourceUpdates(t *testing.T) {
	setupTestDB(t)
	u, requests := serveSource(t, testSource)
	alpha := latestBuild(t, u, "com.example.alpha")
	beta := latestBuild(t, u, "com.example.beta")
	atomic.StoreInt32(requests, 0)

	oldDate := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	manual := createApp(t, trackedApp("u1", "com.example.alpha", u, model.AppSource{BuildID: "old", Version: "1.0", BuildDate: &oldDate}))
	auto := createApp(t, trackedApp("u2", "com.example.beta", strings.ToUpper(u[:4])+u[4:], model.AppSource{AutoUpdate: true}))
	missing := createApp(t, trackedApp("u3", "com.example.gone", u, model.AppSource{}))
	upToDate := createApp(t, trackedApp("u4", "com.example.alpha", u, model.AppSource{BuildID: alpha.ID}))
	untracked := createApp(t, model.InstalledApp{IpaName: "Plain", UDID: "u5", Account: "a@b.c", BundleIdentifier: "com.example.plain"})
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()
	offline := createApp(t, trackedApp("u6", "com.example.alpha", broken.URL+"/apps.json", model.AppSource{LatestBuildID: "x", LatestVersion: "9"}))

	apps, err := GetAppList()
	if err != nil {
		t.Fatal(err)
	}
	res := CheckSourceUpdates(apps)
	if n := atomic.LoadInt32(requests); n != 1 {
		t.Fatalf("%d requests for one source, want 1", n)
	}

	updates := map[uint]model.InstalledApp{}
	for _, v := range res.Updates {
		updates[v.ID] = v
	}
	if len(updates) != 2 || updates[manual.ID].IpaPath != alpha.DownloadURL || updates[manual.ID].Source.BuildID != alpha.ID ||
		updates[auto.ID].Source.BuildID != beta.ID || !updates[auto.ID].Source.AutoUpdate {
		t.Fatalf("unexpected updates %+v", res.Updates)
	}
	wantNotices := []SourceNotice{
		{AppName: "com.example.gone", Error: "no matching build: app com.example.gone not found in the source"},
		{AppName: "com.example.alpha", Version: "2.0"},
	}
	if !sameNotices(res.Notices, wantNotices) {
		t.Fatalf("notices = %+v, want %+v", res.Notices, wantNotices)
	}

	if s := mustGetApp(t, manual.ID).Source; s.CheckedAt == nil || s.LatestBuildID != alpha.ID || s.LatestVersion != "2.0" || s.BuildID != "old" {
		t.Fatalf("manual app source = %+v", s)
	}
	if s := mustGetApp(t, missing.ID).Source; s.CheckedAt == nil || s.CheckError == "" || s.LatestBuildID != "" {
		t.Fatalf("missing app source = %+v", s)
	}
	if s := mustGetApp(t, upToDate.ID).Source; s.CheckedAt == nil || s.LatestBuildID != "" || s.CheckError != "" {
		t.Fatalf("up-to-date app source = %+v", s)
	}
	if s := mustGetApp(t, untracked.ID).Source; s.CheckedAt != nil {
		t.Fatalf("untracked app was checked: %+v", s)
	}
	if s := mustGetApp(t, offline.ID).Source; s.CheckedAt == nil || s.CheckError != "source returned 500" || s.LatestBuildID != "x" {
		t.Fatalf("offline app source = %+v", s)
	}

	// A second run finds the same: nothing new to notify.
	apps, _ = GetAppList()
	res = CheckSourceUpdates(apps)
	if n := atomic.LoadInt32(requests); n != 2 {
		t.Fatalf("%d requests after two runs, want 2", n)
	}
	if len(res.Updates) != 2 || len(res.Notices) != 0 {
		t.Fatalf("second run: %d updates, notices %+v", len(res.Updates), res.Notices)
	}
}

func TestCheckSourceUpdatesNotifiesAutoUpdateWhenRefreshDisabled(t *testing.T) {
	setupTestDB(t)
	u, _ := serveSource(t, testSource)
	createApp(t, trackedApp("u1", "com.example.beta", u, model.AppSource{AutoUpdate: true}))

	// The auto refresh task installs automatic updates: without it the user
	// must be told about them.
	app.Settings.Task.Enabled = false
	apps, _ := GetAppList()
	res := CheckSourceUpdates(apps)
	want := []SourceNotice{{AppName: "com.example.beta", Version: "5.0"}}
	if len(res.Updates) != 1 || !sameNotices(res.Notices, want) {
		t.Fatalf("result = %+v, want notices %+v", res, want)
	}

	// Once is enough.
	apps, _ = GetAppList()
	if res = CheckSourceUpdates(apps); len(res.Notices) != 0 {
		t.Fatalf("second run notices = %+v", res.Notices)
	}
}

func sameNotices(got, want []SourceNotice) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[SourceNotice]int{}
	for _, n := range got {
		seen[n]++
	}
	for _, n := range want {
		seen[n]--
	}
	for _, c := range seen {
		if c != 0 {
			return false
		}
	}
	return true
}

func TestCheckSourceUpdatesRateLimit(t *testing.T) {
	setupTestDB(t)
	u, _ := serveSource(t, testSource)

	var githubRequests int
	limit := &source.RateLimitError{Reset: time.Now().Add(time.Hour)}
	oldFetch := fetchSource
	fetchSource = func(kind, location string) (source.Feed, error) {
		if kind == source.KindGitHub {
			githubRequests++
			return nil, limit
		}
		return source.Fetch(kind, location)
	}
	t.Cleanup(func() { fetchSource = oldFetch })

	github := func(udid, repo, checkError string) model.InstalledApp {
		v := model.InstalledApp{IpaName: repo, UDID: udid, Account: "a@b.c", BundleIdentifier: "com.example." + udid, DeviceClass: "AppleTV"}
		v.Source = model.AppSource{Kind: source.KindGitHub, URL: repo, Filter: "tvos", LatestBuildID: "9", LatestVersion: "v9", CheckError: checkError}
		return createApp(t, v)
	}
	first := github("g1", "owner/one", "earlier")
	second := github("g2", "owner/two", "")
	alt := createApp(t, trackedApp("a1", "com.example.beta", u, model.AppSource{}))

	apps, _ := GetAppList()
	res := CheckSourceUpdates(apps)
	if githubRequests != 1 {
		t.Fatalf("%d GitHub requests, want 1 (the quota is exhausted)", githubRequests)
	}
	// Not checked is not up to date: the rate limit is the check error unless
	// an earlier one is kept, and what the last check found is kept.
	for id, wantErr := range map[uint]string{first.ID: "earlier", second.ID: limit.Error()} {
		s := mustGetApp(t, id).Source
		if s.CheckedAt == nil || s.LatestBuildID != "9" || s.CheckError != wantErr {
			t.Fatalf("rate-limited app %d source = %+v, want check error %q", id, s, wantErr)
		}
	}
	if s := mustGetApp(t, alt.ID).Source; s.CheckedAt == nil || s.LatestBuildID == "" {
		t.Fatalf("AltStore app not checked: %+v", s)
	}
	if len(res.Updates) != 1 || res.Updates[0].ID != alt.ID || len(res.Notices) != 1 {
		t.Fatalf("unexpected result %+v", res)
	}
}

func TestCheckSourceUpdatesNotifiesMatchErrorOnceAcrossFetchErrors(t *testing.T) {
	setupTestDB(t)
	u, _ := serveSource(t, testSource)
	var offline bool
	oldFetch := fetchSource
	fetchSource = func(kind, location string) (source.Feed, error) {
		if offline {
			return nil, errors.New("timeout")
		}
		return source.Fetch(kind, location)
	}
	t.Cleanup(func() { fetchSource = oldFetch })
	gone := createApp(t, trackedApp("u1", "com.example.gone", u, model.AppSource{}))

	// A network blip between two checks finding the same problem is no news.
	var notices []SourceNotice
	for _, down := range []bool{false, true, false} {
		offline = down
		apps, _ := GetAppList()
		notices = append(notices, CheckSourceUpdates(apps).Notices...)
	}
	matchErr := "no matching build: app com.example.gone not found in the source"
	if want := []SourceNotice{{AppName: "com.example.gone", Error: matchErr}}; !sameNotices(notices, want) {
		t.Fatalf("notices = %+v, want %+v", notices, want)
	}
	if s := mustGetApp(t, gone.ID).Source; s.CheckError != matchErr {
		t.Fatalf("check error = %q, want %q", s.CheckError, matchErr)
	}
}

func TestResolveSourceInstall(t *testing.T) {
	u, _ := serveSource(t, testSource)
	alpha := latestBuild(t, u, "com.example.alpha")

	v := model.InstalledApp{IpaName: "shown", IpaPath: "https://attacker.example/evil.ipa"}
	v.Source = model.AppSource{
		Kind: source.KindAltStore, URL: " " + u + " ", Filter: "com.example.alpha", AutoUpdate: true, BuildID: alpha.ID,
		CheckedAt: timePtr(time.Now()), CheckError: "client", LatestBuildID: "x", FailedBuildID: "y",
	}
	if err := ResolveSourceInstall(&v); err != nil {
		t.Fatalf("ResolveSourceInstall: %v", err)
	}
	want := model.AppSource{
		Kind: source.KindAltStore, URL: u, Filter: "com.example.alpha", AutoUpdate: true,
		BuildID: alpha.ID, Version: "2.0", BuildName: "Alpha-2.0.ipa", BuildDate: &alpha.Date,
	}
	if v.IpaPath != alpha.DownloadURL {
		t.Fatalf("IpaPath = %q, want %q", v.IpaPath, alpha.DownloadURL)
	}
	assertSource(t, v.Source, want)

	bad := model.InstalledApp{}
	bad.Source = model.AppSource{Kind: source.KindAltStore, URL: u, Filter: "com.example.beta", BuildID: alpha.ID}
	if err := ResolveSourceInstall(&bad); err == nil || err.Error() != "the filter does not match Alpha" {
		t.Fatalf("mismatched filter: %v", err)
	}
	bad.Source = model.AppSource{Kind: "gitlab", URL: u, Filter: "x", BuildID: alpha.ID}
	if err := ResolveSourceInstall(&bad); err == nil {
		t.Fatal("unknown kind should fail")
	}
}

func TestLinkUpdateAndUntrackSource(t *testing.T) {
	setupTestDB(t)
	u, _ := serveSource(t, testSource)
	alpha := latestBuild(t, u, "com.example.alpha")
	installed := createApp(t, model.InstalledApp{IpaName: "Alpha", UDID: "u1", Account: "a@b.c", BundleIdentifier: "com.example.alpha", DeviceClass: "AppleTV"})

	if _, err := LinkSource(installed.ID, SourceInput{Kind: source.KindAltStore, URL: u, Filter: "com.example.beta"}); err == nil ||
		err.Error() != "the source app com.example.beta does not match the installed bundle com.example.alpha" {
		t.Fatalf("mismatched bundle: %v", err)
	}

	// Linked without knowing the installed build: the newest one is an update.
	linked, err := LinkSource(installed.ID, SourceInput{Kind: source.KindAltStore, URL: u, Filter: "com.example.alpha", AutoUpdate: true})
	if err != nil {
		t.Fatalf("LinkSource: %v", err)
	}
	if s := linked.Source; s.Kind != source.KindAltStore || s.URL != u || !s.AutoUpdate || s.BuildID != "" || s.LatestBuildID != alpha.ID || s.CheckedAt == nil {
		t.Fatalf("linked source = %+v", s)
	}
	tracked, err := GetTrackedAppList()
	if err != nil || len(tracked) != 1 {
		t.Fatalf("GetTrackedAppList = %d apps, %v", len(tracked), err)
	}

	target, err := PrepareSourceUpdate(installed.ID, "", "")
	if err != nil {
		t.Fatalf("PrepareSourceUpdate: %v", err)
	}
	if target.ID != installed.ID || target.IpaPath != alpha.DownloadURL || target.Source.BuildID != alpha.ID || target.Source.LatestBuildID != "" {
		t.Fatalf("update target = %+v", target)
	}
	if _, err := PrepareSourceUpdate(installed.ID, "", "com.example.beta"); err == nil {
		t.Fatal("a filter selecting another app should fail")
	}
	if got := mustGetApp(t, installed.ID).Source.Filter; got != "com.example.alpha" {
		t.Fatalf("filter changed to %q by a failed update", got)
	}

	if err := MarkSourceUpdateFailed(installed.ID, alpha.ID, "boom"); err != nil {
		t.Fatal(err)
	}
	if s := mustGetApp(t, installed.ID).Source; s.FailedBuildID != alpha.ID || s.CheckError != "update failed: boom" || s.LatestBuildID != alpha.ID {
		t.Fatalf("failed update source = %+v", s)
	}

	// The installed version is already the newest build.
	linked, err = LinkSource(installed.ID, SourceInput{Kind: source.KindAltStore, URL: u, Filter: "com.example.alpha", InstalledBuildID: alpha.ID})
	if err != nil {
		t.Fatalf("LinkSource: %v", err)
	}
	if s := linked.Source; s.BuildID != alpha.ID || s.Version != "2.0" || s.BuildName != "Alpha-2.0.ipa" || s.LatestBuildID != "" || s.FailedBuildID != "" || s.AutoUpdate {
		t.Fatalf("linked source = %+v", s)
	}

	if err := UntrackSource(installed.ID); err != nil {
		t.Fatal(err)
	}
	assertSource(t, mustGetApp(t, installed.ID).Source, model.AppSource{})
	if _, err := PrepareSourceUpdate(installed.ID, "", ""); err == nil {
		t.Fatal("updating an untracked app should fail")
	}
}

func TestSourceKeepsFilterOfAnotherBundle(t *testing.T) {
	setupTestDB(t)
	u, _ := serveSource(t, testSource)
	alpha := latestBuild(t, u, "com.example.alpha")
	// Installed from the source, which lists another bundle identifier than
	// the IPA has.
	v := trackedApp("u1", "com.example.alpha", u, model.AppSource{})
	v.BundleIdentifier = "com.example.alpha.tvos"
	installed := createApp(t, v)

	// Saving the settings or updating with the stored filter keeps working...
	linked, err := LinkSource(installed.ID, SourceInput{Kind: source.KindAltStore, URL: u, Filter: "com.example.alpha", AutoUpdate: true})
	if err != nil {
		t.Fatalf("LinkSource: %v", err)
	}
	if s := linked.Source; !s.AutoUpdate || s.Filter != "com.example.alpha" || s.LatestBuildID != alpha.ID {
		t.Fatalf("linked source = %+v", s)
	}
	target, err := PrepareSourceUpdate(installed.ID, "", "com.example.alpha")
	if err != nil {
		t.Fatalf("PrepareSourceUpdate: %v", err)
	}
	if target.IpaPath != alpha.DownloadURL || target.Source.Filter != "com.example.alpha" {
		t.Fatalf("update target = %+v", target)
	}

	// ...while selecting another app is still rejected.
	wantErr := "the source app com.example.beta does not match the installed bundle com.example.alpha.tvos"
	if _, err := LinkSource(installed.ID, SourceInput{Kind: source.KindAltStore, URL: u, Filter: "com.example.beta"}); err == nil || err.Error() != wantErr {
		t.Fatalf("LinkSource with another app: %v", err)
	}
	if _, err := PrepareSourceUpdate(installed.ID, "", "com.example.beta"); err == nil || err.Error() != wantErr {
		t.Fatalf("PrepareSourceUpdate with another app: %v", err)
	}
}

// variantFeed is a GitHub release with two variants of the same app.
type variantFeed struct{ builds []source.Build }

func (f variantFeed) Preview(string, bool) (*source.Preview, error) { return nil, nil }

func (f variantFeed) Latest(filter string, _ bool, _ string) (source.Build, error) {
	for _, b := range f.builds {
		if source.FilterMatches(source.KindGitHub, filter, b) {
			return b, nil
		}
	}
	return source.Build{}, source.ErrNoMatch
}

func (f variantFeed) Find(id string) (source.Build, error) {
	for _, b := range f.builds {
		if b.ID == id {
			return b, nil
		}
	}
	return source.Build{}, source.ErrNoMatch
}

func TestPrepareSourceUpdateKeepsFilterUntilInstalled(t *testing.T) {
	setupTestDB(t)
	names := []string{"OrivioTV-V9.Sideload.ipa", "OrivioTV-V9.Sideloadly.ipa"}
	feed := variantFeed{}
	for i, n := range names {
		feed.builds = append(feed.builds, source.Build{
			ID:          fmt.Sprint(1001 + i),
			Name:        n,
			Version:     "v0.10",
			DownloadURL: "https://github.com/prehakanson-art/OrivioTVAppleTV/releases/download/v0.10/" + n,
			Filter:      source.DeriveFilter(n, names),
		})
	}
	oldFetch := fetchSource
	fetchSource = func(string, string) (source.Feed, error) { return feed, nil }
	t.Cleanup(func() { fetchSource = oldFetch })

	sideload, sideloadly := feed.builds[0], feed.builds[1]
	v := model.InstalledApp{IpaName: "Orivio TV", UDID: "u1", Account: "a@b.c", BundleIdentifier: "com.orivio.tv.appletv.dev", DeviceClass: "AppleTV"}
	v.Source = model.AppSource{Kind: source.KindGitHub, URL: "prehakanson-art/OrivioTVAppleTV", Filter: sideload.Filter}
	installed := createApp(t, v)

	// Switching to the other variant: the new filter travels with the update...
	target, err := PrepareSourceUpdate(installed.ID, sideloadly.ID, sideloadly.Filter)
	if err != nil {
		t.Fatalf("PrepareSourceUpdate: %v", err)
	}
	if target.Source.Filter != sideloadly.Filter || target.IpaPath != sideloadly.DownloadURL {
		t.Fatalf("update target source = %+v", target.Source)
	}
	// ...but the stored filter keeps tracking the installed variant until the
	// update succeeds (the variants may be different apps).
	if got := mustGetApp(t, installed.ID).Source.Filter; got != sideload.Filter {
		t.Fatalf("stored filter = %q, want %q", got, sideload.Filter)
	}

	// The source dialog's Install with auto-update turned on: it saves the
	// settings with the stored filter, then installs the other variant, which
	// is rejected (another bundle identifier).
	if _, err := LinkSource(installed.ID, SourceInput{Kind: source.KindGitHub, URL: "prehakanson-art/OrivioTVAppleTV", Filter: sideload.Filter, AutoUpdate: true}); err != nil {
		t.Fatalf("LinkSource: %v", err)
	}
	if _, err := PrepareSourceUpdate(installed.ID, sideloadly.ID, sideloadly.Filter); err != nil {
		t.Fatalf("PrepareSourceUpdate: %v", err)
	}
	if err := MarkSourceUpdateFailed(installed.ID, sideloadly.ID, "bundle identifier changed"); err != nil {
		t.Fatal(err)
	}
	if s := mustGetApp(t, installed.ID).Source; s.Filter != sideload.Filter || !s.AutoUpdate || s.FailedBuildID != sideloadly.ID {
		t.Fatalf("source after a failed switch = %+v, want the %q filter", s, sideload.Filter)
	}
}
