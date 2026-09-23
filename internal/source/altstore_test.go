package source

import (
	"errors"
	"net/http"
	"net/http/httptest"
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
		b.Platform != PlatformTVOS || b.Filter != "com.pyksel.nuviotvos" || b.Prerelease {
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
	if older.Version != "3.3.6 (42)" || older.Size != 23900000 || !older.Date.Equal(time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)) {
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
	if _, err := feed.Latest("com.example.app", false, "AppleTV"); !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("Latest = %v, want ErrAmbiguous", err)
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
