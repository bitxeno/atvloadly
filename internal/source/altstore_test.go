package source

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const nuvioSource = `{
  "name": "bobsupra's NuvioTVOS",
  "identifier": "com.bobsupra.nuviotvos.source",
  "apps": [{
    "name": "NuvioTVOS",
    "bundleIdentifier": "com.pyksel.nuviotvos",
    "developerName": "bobsupra",
    "subtitle": "Nuvio TV for tvOS",
    "iconURL": "https://example.com/nuviotvos-icon.png",
    "localizedDescription": "Nuvio for Apple TV",
    "versions": [
      {"version": "3.3.7", "date": "2026-09-21T21:12:00Z", "downloadURL": "https://github.com/bobsupra/NuvioTVOS/releases/download/tvos-beta-3.3.7/NuvioTV-3.3.7-unsigned-release.ipa", "size": 23972092, "localizedDescription": "..."},
      {"version": "3.3.6", "buildVersion": "42", "date": "2026-09-10", "downloadURL": "https://github.com/bobsupra/NuvioTVOS/releases/download/tvos-beta-3.3.6/NuvioTV-3.3.6-unsigned-release.ipa", "size": "23900000"},
      {"version": "3.3.5", "date": "yesterday", "size": 1}
    ]
  }]
}`

func serveSource(t *testing.T, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing.json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/apps.json"
}

func TestNormalizeAltStore(t *testing.T) {
	u := "https://raw.githubusercontent.com/bobsupra/NuvioTVOS/main/apps.json"
	if got, err := Normalize(KindAltStore, "  "+u+"\n"); err != nil || got != u {
		t.Errorf("Normalize = %q, %v", got, err)
	}
	for _, bad := range []string{"", "apps.json", "ftp://example.com/apps.json", "https://", "bobsupra/NuvioTVOS"} {
		if got, err := Normalize(KindAltStore, bad); err == nil {
			t.Errorf("Normalize(%q) = %q, want an error", bad, got)
		}
	}
	if _, err := Normalize("gitlab", u); err == nil {
		t.Error("unknown kind should be rejected")
	}
}

func TestAltStoreNuvio(t *testing.T) {
	u := serveSource(t, nuvioSource)
	feed, err := Fetch(KindAltStore, u)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	p, err := feed.Preview("AppleTV", false)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if p.Kind != KindAltStore || p.URL != u || p.PageURL != u || p.Title != "bobsupra's NuvioTVOS" || len(p.Builds) != 1 {
		t.Fatalf("unexpected preview %+v", p)
	}
	b := p.Builds[0]
	if p.SuggestedID != b.ID || len(b.ID) != 16 {
		t.Fatalf("unexpected suggestion %q for %q", p.SuggestedID, b.ID)
	}
	wantDate := time.Date(2026, 9, 21, 21, 12, 0, 0, time.UTC)
	if b.Name != "NuvioTVOS" || b.Version != "3.3.7" || !b.Date.Equal(wantDate) || b.Size != 23972092 ||
		b.DownloadURL != "https://github.com/bobsupra/NuvioTVOS/releases/download/tvos-beta-3.3.7/NuvioTV-3.3.7-unsigned-release.ipa" ||
		b.PageURL != "" || b.IconURL != "https://example.com/nuviotvos-icon.png" || b.BundleID != "com.pyksel.nuviotvos" ||
		b.Developer != "bobsupra" || b.Subtitle != "Nuvio TV for tvOS" || b.Platform != PlatformTVOS || b.Filter != "com.pyksel.nuviotvos" || b.Prerelease {
		t.Fatalf("unexpected build %+v", b)
	}

	latest, err := feed.Latest("com.pyksel.nuviotvos", false, "AppleTV")
	if err != nil || latest.ID != b.ID {
		t.Fatalf("Latest = %+v, %v", latest, err)
	}
	if _, err := feed.Latest("com.other.app", false, "AppleTV"); !errors.Is(err, ErrNoMatch) {
		t.Fatalf("Latest(other) = %v, want ErrNoMatch", err)
	}

	if found, err := feed.Find(b.ID); err != nil || found.ID != b.ID || found.Version != b.Version {
		t.Fatalf("Find(%s) = %+v, %v", b.ID, found, err)
	}

	// The version without downloadURL is skipped; the second one has a
	// string size, a date without time and a build number.
	builds := feed.(*altStoreFeed).entries[0].builds
	if len(builds) != 2 {
		t.Fatalf("got %d builds, want 2", len(builds))
	}
	older, err := feed.Find(builds[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	if older.Version != "3.3.6 (42)" || older.Size != 23900000 || !older.Date.Equal(time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)) ||
		older.Developer != "bobsupra" || older.Subtitle != "Nuvio TV for tvOS" {
		t.Fatalf("unexpected older build %+v", older)
	}
	if _, err := feed.Find("0000000000000000"); !errors.Is(err, ErrNoMatch) {
		t.Fatalf("Find(missing) = %v, want ErrNoMatch", err)
	}

	// IDs are stable across fetches.
	again, err := Fetch(KindAltStore, u)
	if err != nil {
		t.Fatal(err)
	}
	if b2, err := again.Latest("com.pyksel.nuviotvos", false, ""); err != nil || b2.ID != b.ID {
		t.Fatalf("second fetch Latest = %+v, %v", b2, err)
	}
}

func TestAltStoreLegacyApp(t *testing.T) {
	// Some sources are saved with a UTF-8 byte order mark.
	u := serveSource(t, "\ufeff"+`{"name":"Legacy","apps":[
		{"name":"Old App","bundleIdentifier":"com.example.old","version":"1.0","versionDate":"2024-01-02T03:04:05","downloadURL":"https://example.com/Old-iOS.ipa","size":"1024"},
		{"name":"Nothing","bundleIdentifier":"com.example.none","version":"1.0"}
	]}`)
	feed, err := Fetch(KindAltStore, u)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	b, err := feed.Latest("com.example.old", false, "")
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if b.Version != "1.0" || b.Size != 1024 || b.Platform != PlatformIOS ||
		!b.Date.Equal(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Fatalf("unexpected build %+v", b)
	}
	if _, err := feed.Latest("com.example.none", false, ""); !errors.Is(err, ErrNoMatch) {
		t.Fatalf("Latest(no version) = %v, want ErrNoMatch", err)
	}

	p, err := feed.Preview("AppleTV", false)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if p.Title != "Legacy" {
		t.Fatalf("Title = %q, want %q", p.Title, "Legacy")
	}
	// A single app that looks built for iOS is listed but not suggested for an Apple TV.
	if len(p.Builds) != 1 || p.SuggestedID != "" {
		t.Fatalf("unexpected preview %+v", p)
	}
	if p, err = feed.Preview("iPhone", false); err != nil || p.SuggestedID != b.ID {
		t.Fatalf("Preview(iPhone) = %+v, %v; want %q suggested", p, err, b.ID)
	}
}

func TestAltStoreSeveralApps(t *testing.T) {
	u := serveSource(t, `{"name":"Multi","apps":[
		{"name":"App iOS","bundleIdentifier":"com.example.app","versions":[{"version":"2","date":"2025-05-01","downloadURL":"https://example.com/App-iOS.ipa"}]},
		{"name":"App TV","bundleIdentifier":"com.example.app","subtitle":"for Apple TV","versions":[{"version":"2","date":"not a date","downloadURL":"https://example.com/App-tvOS.ipa"}]}
	]}`)
	feed, err := Fetch(KindAltStore, u)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	p, err := feed.Preview("AppleTV", false)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if len(p.Builds) != 2 || p.SuggestedID != p.Builds[1].ID || !p.Builds[1].Date.IsZero() {
		t.Fatalf("unexpected preview %+v", p)
	}
	// The platform of the device picks the app among those using the bundle.
	for deviceClass, want := range map[string]string{"AppleTV": "App TV", "iPhone": "App iOS"} {
		if b, err := feed.Latest("com.example.app", false, deviceClass); err != nil || b.Name != want {
			t.Fatalf("Latest(%s) = %+v, %v; want %s", deviceClass, b, err, want)
		}
	}
	if _, err := feed.Latest("com.example.app", false, ""); !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("Latest(unknown device) = %v, want ErrAmbiguous", err)
	}

	// Two channels for the same platform stay ambiguous.
	u = serveSource(t, `{"name":"Channels","apps":[
		{"name":"App","bundleIdentifier":"com.example.app","subtitle":"for tvOS","versions":[{"version":"2","downloadURL":"https://example.com/App-tvOS.ipa"}]},
		{"name":"App Beta","bundleIdentifier":"com.example.app","subtitle":"for tvOS","versions":[{"version":"3b","downloadURL":"https://example.com/App-beta-tvOS.ipa"}]}
	]}`)
	if feed, err = Fetch(KindAltStore, u); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if _, err := feed.Latest("com.example.app", false, "AppleTV"); !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("Latest = %v, want ErrAmbiguous", err)
	}
}

func TestAltStoreSkipsLocalDownloads(t *testing.T) {
	u := serveSource(t, `{"name":"Local","apps":[{"name":"App","bundleIdentifier":"com.example.app","versions":[
		{"version":"4","downloadURL":"/data/ipa/1/app.ipa"},
		{"version":"3","downloadURL":"app.ipa"},
		{"version":"2","downloadURL":" https://example.com/App-2.ipa"},
		{"version":"1.5","downloadURL":"file:///data/ipa/1/app.ipa"},
		{"version":"1","downloadURL":"HTTPS://example.com/App-1.ipa"}
	]}]}`)
	feed, err := Fetch(KindAltStore, u)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	// Only http(s) URLs are downloads; the others would be read on the server.
	builds := feed.(*altStoreFeed).entries[0].builds
	if len(builds) != 1 || builds[0].Version != "1" {
		t.Fatalf("builds = %+v, want only version 1", builds)
	}
	if b, err := feed.Latest("com.example.app", false, "AppleTV"); err != nil || b.DownloadURL != "HTTPS://example.com/App-1.ipa" {
		t.Fatalf("Latest = %+v, %v", b, err)
	}
}

func TestAltStoreErrors(t *testing.T) {
	u := serveSource(t, `{"apps":[]}`)
	feed, err := Fetch(KindAltStore, u)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if _, err := feed.Preview("AppleTV", false); !errors.Is(err, ErrNoMatch) {
		t.Fatalf("Preview(empty) = %v, want ErrNoMatch", err)
	}

	missing := u[:len(u)-len("/apps.json")] + "/missing.json"
	if _, err := Fetch(KindAltStore, missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Fetch(404) = %v, want ErrNotFound", err)
	}

	bad := serveSource(t, `<html>not json</html>`)
	if _, err := Fetch(KindAltStore, bad); err == nil {
		t.Fatal("Fetch(html) should fail")
	}
}

// TestAltStoreSizeLimit checks that a source larger than the cap, such as an
// IPA pasted as a source URL, is rejected instead of being read into memory.
func TestAltStoreSizeLimit(t *testing.T) {
	defer func(n int64) { maxAltStoreSize = n }(maxAltStoreSize)
	maxAltStoreSize = 1 << 20

	src := `{"name":"Big","apps":[]}` + strings.Repeat(" ", int(maxAltStoreSize)-len(`{"name":"Big","apps":[]}`))
	if _, err := Fetch(KindAltStore, serveSource(t, src)); err != nil {
		t.Fatalf("Fetch(at the cap) = %v", err)
	}
	if _, err := Fetch(KindAltStore, serveSource(t, src+" ")); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("Fetch(over the cap) = %v, want a size error", err)
	}
}

func TestParseAltStoreDate(t *testing.T) {
	est := time.FixedZone("", -5*3600)
	cases := []struct {
		in   string
		want time.Time
	}{
		{"2023-2-17", time.Date(2023, 2, 17, 0, 0, 0, 0, time.UTC)},
		{"2023-2-02T03:00:00-05:00", time.Date(2023, 2, 2, 3, 0, 0, 0, est)},
		{"2026-09-21T21:12:00Z", time.Date(2026, 9, 21, 21, 12, 0, 0, time.UTC)},
		{"2026-09-21T21:12:00.250-05:00", time.Date(2026, 9, 21, 21, 12, 0, 250e6, est)},
		{"2026-09-10", time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)},
		{"2026-09-10T08:30:00", time.Date(2026, 9, 10, 8, 30, 0, 0, time.UTC)},
		{"yesterday", time.Time{}},
		{"", time.Time{}},
	}
	for _, c := range cases {
		if got := parseAltStoreDate(c.in); !got.Equal(c.want) {
			t.Errorf("parseAltStoreDate(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
