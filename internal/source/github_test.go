package source

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestParseRepo(t *testing.T) {
	valid := map[string]string{
		"prehakanson-art/OrivioTVAppleTV":                                       "prehakanson-art/OrivioTVAppleTV",
		" bobsupra/NuvioTVOS ":                                                  "bobsupra/NuvioTVOS",
		"https://github.com/owner/repo":                                         "owner/repo",
		"github.com/owner/repo/":                                                "owner/repo",
		"https://www.github.com/owner/repo.git":                                 "owner/repo",
		"https://github.com/prehakanson-art/OrivioTVAppleTV/releases/tag/v0.10": "prehakanson-art/OrivioTVAppleTV",
		"http://GitHub.com/owner/my.repo_name?tab=readme#top":                   "owner/my.repo_name",
	}
	for input, want := range valid {
		got, err := ParseRepo(input)
		if err != nil || got != want {
			t.Errorf("ParseRepo(%q) = %q, %v; want %q", input, got, err, want)
		}
		if n, err := Normalize(KindGitHub, input); err != nil || n != want {
			t.Errorf("Normalize(github, %q) = %q, %v; want %q", input, n, err, want)
		}
	}

	invalid := []string{
		"",
		"owner",
		"https://gitlab.com/owner/repo",
		"https://raw.githubusercontent.com/bobsupra/NuvioTVOS/main/apps.json",
		"ftp://github.com/owner/repo",
		"github.com/owner",
		"-owner/repo",
		"owner_name/repo",
		"owner/..",
		"owner/re po",
	}
	for _, input := range invalid {
		if got, err := ParseRepo(input); err == nil {
			t.Errorf("ParseRepo(%q) = %q, want an error", input, got)
		}
	}
}

func asset(id int64, name string) githubAsset {
	return githubAsset{
		ID:                 id,
		Name:               name,
		State:              "uploaded",
		Size:               52100000,
		CreatedAt:          time.Date(2026, 9, 21, 17, 13, 0, 0, time.UTC).Add(time.Duration(id) * time.Minute),
		BrowserDownloadURL: "https://github.com/owner/repo/releases/download/x/" + name,
	}
}

func release(tag string, prerelease bool, assets ...githubAsset) githubRelease {
	return githubRelease{
		TagName:    tag,
		Prerelease: prerelease,
		HTMLURL:    "https://github.com/owner/repo/releases/tag/" + tag,
		Assets:     assets,
	}
}

// orivioReleases mirrors prehakanson-art/OrivioTVAppleTV: the tag, the file
// number and the app version are unrelated.
func orivioReleases() []githubRelease {
	return []githubRelease{
		release("v0.10", false, asset(301, "OrivioTV-V9.Sideload.ipa"), asset(302, "OrivioTV-V9.Sideloadly.ipa")),
		release("v0.07", false, asset(201, "OrivioTV-V7.Sideload.ipa"), asset(202, "OrivioTV-V7.Sideloadly.ipa")),
		release("v0.04", false, asset(101, "OrivioTV-0.7.15-tvos-unsigned.ipa"), asset(102, "OrivioTV-0.7.15-sideloadly-unsigned.ipa")),
		release("v0.02", false, asset(11, "OrivioTV-V2.Sideloadly.ipa"), asset(12, "OrivioTV-0.7.15-tvos.ipa")),
	}
}

// serveReleases serves releases as the GitHub API.
func serveReleases(t *testing.T, releases []githubRelease) {
	t.Helper()
	data, err := json.Marshal(releases)
	if err != nil {
		t.Fatal(err)
	}
	serveGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	})
}

// serveGitHub points the GitHub API to a local server running handler.
func serveGitHub(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	old := apiBaseURL
	apiBaseURL = srv.URL
	t.Cleanup(func() { apiBaseURL = old })
}

func TestFetchGitHubDecode(t *testing.T) {
	data, _ := json.Marshal(orivioReleases())
	serveGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/prehakanson-art/OrivioTVAppleTV/releases" || r.URL.Query().Get("per_page") != "30" {
			t.Errorf("unexpected request %s", r.URL)
		}
		if r.Header.Get("Accept") != "application/vnd.github+json" || r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
			t.Errorf("unexpected headers %v", r.Header)
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent")
		}
		_, _ = w.Write(data)
	})

	feed, err := Fetch(KindGitHub, "prehakanson-art/OrivioTVAppleTV")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	p, err := feed.Preview("AppleTV", false)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if p.Kind != KindGitHub || p.URL != "prehakanson-art/OrivioTVAppleTV" || p.Title != "v0.10" ||
		p.PageURL != "https://github.com/owner/repo/releases/tag/v0.10" || p.SuggestedID != "" {
		t.Fatalf("unexpected preview %+v", p)
	}
	if len(p.Builds) != 2 || p.Builds[0].ID != "301" || p.Builds[1].ID != "302" {
		t.Fatalf("unexpected builds %+v", p.Builds)
	}
	b := p.Builds[0]
	if b.Name != "OrivioTV-V9.Sideload.ipa" || b.Version != "v0.10" || b.Size != 52100000 || b.Platform != PlatformUnknown ||
		b.DownloadURL != "https://github.com/owner/repo/releases/download/x/OrivioTV-V9.Sideload.ipa" ||
		b.Filter != `(?i)(^|[^a-z0-9])sideload([^a-z0-9]|$)` || b.Date.IsZero() {
		t.Fatalf("unexpected build %+v", b)
	}
}

func TestFetchGitHubErrors(t *testing.T) {
	serveGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	})
	if _, err := Fetch(KindGitHub, "owner/missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("404: got %v, want ErrNotFound", err)
	}

	reset := time.Now().Add(20 * time.Minute).Truncate(time.Second)
	serveGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset.Unix(), 10))
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	})
	_, err := Fetch(KindGitHub, "owner/repo")
	var rl *RateLimitError
	if !errors.As(err, &rl) || !rl.Reset.Equal(reset) {
		t.Errorf("403 rate limit: got %v", err)
	}

	serveGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	_, err = Fetch(KindGitHub, "owner/repo")
	if !errors.As(err, &rl) || time.Until(rl.Reset) < 30*time.Second {
		t.Errorf("429 rate limit: got %v", err)
	}

	serveGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Repository access blocked"}`))
	})
	_, err = Fetch(KindGitHub, "owner/repo")
	if err == nil || errors.As(err, &rl) || err.Error() != "GitHub API returned 403: Repository access blocked" {
		t.Errorf("403 without rate limit: got %v", err)
	}
}

func TestRateLimitErrorMessage(t *testing.T) {
	reset := time.Date(2026, 9, 23, 15, 4, 0, 0, time.Local)
	if got := (&RateLimitError{Reset: reset}).Error(); got != "GitHub API rate limit exceeded, retry after 15:04" {
		t.Errorf("Error() = %q", got)
	}
}

func TestGitHubPreviewSkipsOtherPlatformRelease(t *testing.T) {
	serveReleases(t, []githubRelease{
		release("v2", false, asset(21, "App-iOS.ipa"), asset(22, "Source.zip")),
		release("v1", false, asset(11, "App-iOS.ipa"), asset(12, "App-tvOS.ipa")),
	})
	feed, err := Fetch(KindGitHub, "owner/repo")
	if err != nil {
		t.Fatal(err)
	}

	p, err := feed.Preview("AppleTV", false)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if p.Title != "v1" || p.SuggestedID != "12" || len(p.Builds) != 2 || p.Builds[0].ID != "12" || p.Builds[1].ID != "11" {
		t.Fatalf("AppleTV preview = %+v", p)
	}

	p, err = feed.Preview("iPhone", false)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if p.Title != "v2" || p.SuggestedID != "21" || len(p.Builds) != 1 {
		t.Fatalf("iPhone preview = %+v", p)
	}

	// The iOS-only v2 is skipped for an Apple TV, the iOS IPA of v1 too.
	b, err := feed.Latest(`(?i)app`, false, "AppleTV")
	if err != nil || b.ID != "12" {
		t.Fatalf("Latest on AppleTV = %+v, %v", b, err)
	}
	b, err = feed.Latest(`(?i)app`, false, "iPhone")
	if err != nil || b.ID != "21" {
		t.Fatalf("Latest on iPhone = %+v, %v", b, err)
	}
}

func TestGitHubPreviewNoIPA(t *testing.T) {
	serveReleases(t, []githubRelease{release("v1", false, asset(1, "App.zip"))})
	feed, err := Fetch(KindGitHub, "owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := feed.Preview("AppleTV", false); !errors.Is(err, ErrNoMatch) {
		t.Fatalf("Preview = %v, want ErrNoMatch", err)
	}
}

func TestGitHubLatest(t *testing.T) {
	sideload := DeriveFilter("OrivioTV-V9.Sideload.ipa", []string{"OrivioTV-V9.Sideloadly.ipa"})
	sideloadly := DeriveFilter("OrivioTV-V9.Sideloadly.ipa", []string{"OrivioTV-V9.Sideload.ipa"})

	cases := []struct {
		name     string
		releases []githubRelease
		filter   string
		pre      bool
		wantID   string
		wantErr  error
	}{
		{
			name:     "single match",
			releases: orivioReleases(),
			filter:   sideload,
			wantID:   "301",
		},
		{
			name:     "other variant",
			releases: orivioReleases(),
			filter:   sideloadly,
			wantID:   "302",
		},
		{
			name: "renamed files need attention",
			releases: append([]githubRelease{
				release("v0.11", false, asset(401, "OrivioTV-V10-Beta.ipa"), asset(402, "OrivioTV-V10-Stable.ipa")),
			}, orivioReleases()...),
			filter:  sideload,
			wantErr: ErrNoMatch,
		},
		{
			name: "ambiguity resolved by platform",
			releases: []githubRelease{
				release("v3", false, asset(31, "App-tvOS.ipa"), asset(32, "App-universal.ipa"), asset(33, "App-iOS.ipa")),
			},
			filter: `(?i)app`,
			wantID: "31",
		},
		{
			name: "ambiguous",
			releases: []githubRelease{
				release("v3", false, asset(31, "App-Sideload.ipa"), asset(32, "App-Sideloadly.ipa")),
			},
			filter:  `(?i)sideload`,
			wantErr: ErrAmbiguous,
		},
		{
			name: "pre-release skipped",
			releases: append([]githubRelease{
				release("v0.11-beta", true, asset(501, "OrivioTV-V10.Sideload.ipa")),
				{TagName: "v0.12", Draft: true, Assets: []githubAsset{asset(601, "OrivioTV-V11.Sideload.ipa")}},
			}, orivioReleases()...),
			filter: sideload,
			wantID: "301",
		},
		{
			name: "pre-release included",
			releases: append([]githubRelease{
				release("v0.11-beta", true, asset(501, "OrivioTV-V10.Sideload.ipa")),
				{TagName: "v0.12", Draft: true, Assets: []githubAsset{asset(601, "OrivioTV-V11.Sideload.ipa")}},
			}, orivioReleases()...),
			filter: sideload,
			pre:    true,
			wantID: "501",
		},
		{
			name:     "no release",
			releases: nil,
			filter:   sideload,
			wantErr:  ErrNoMatch,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			serveReleases(t, c.releases)
			feed, err := Fetch(KindGitHub, "owner/repo")
			if err != nil {
				t.Fatal(err)
			}
			b, err := feed.Latest(c.filter, c.pre, "AppleTV")
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("Latest = %+v, %v; want %v", b, err, c.wantErr)
				}
				return
			}
			if err != nil || b.ID != c.wantID {
				t.Fatalf("Latest = %+v, %v; want %s", b, err, c.wantID)
			}
		})
	}

	if _, err := (&githubFeed{}).Latest("(", false, "AppleTV"); err == nil {
		t.Error("invalid filter should fail")
	}
}

func TestGitHubFind(t *testing.T) {
	serveReleases(t, append([]githubRelease{
		release("v0.11-beta", true, asset(501, "OrivioTV-V10.Sideload.ipa")),
	}, orivioReleases()...))
	feed, err := Fetch(KindGitHub, "owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"501", "201", "12"} {
		if b, err := feed.Find(id); err != nil || b.ID != id {
			t.Errorf("Find(%s) = %+v, %v", id, b, err)
		}
	}
	b, _ := feed.Find("11")
	if b.Name != "OrivioTV-V2.Sideloadly.ipa" || b.Version != "v0.02" {
		t.Errorf("Find(11) = %+v", b)
	}
	if _, err := feed.Find("999"); !errors.Is(err, ErrNoMatch) {
		t.Errorf("Find(999) = %v, want ErrNoMatch", err)
	}
}
