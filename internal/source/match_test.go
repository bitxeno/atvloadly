package source

import (
	"reflect"
	"regexp"
	"testing"
)

func TestIsIPAName(t *testing.T) {
	cases := map[string]bool{
		"App.ipa":           true,
		"App.IPA":           true,
		"App.tipa":          true,
		"App.zip":           false,
		"App.ipa.zip":       false,
		"Source code (zip)": false,
	}
	for name, want := range cases {
		if got := IsIPAName(name); got != want {
			t.Errorf("IsIPAName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestTokens(t *testing.T) {
	cases := map[string][]string{
		"OrivioTV-V9.Sideloadly.ipa":         {"oriviotv", "v9", "sideloadly"},
		"OrivioTV-0.7.15-tvos-unsigned.ipa":  {"oriviotv", "0", "7", "15", "tvos", "unsigned"},
		"Swiftfin_tvOS_1.2.ipa":              {"swiftfin", "tvos", "1", "2"},
		"NuvioTV-3.3.7-unsigned-release.ipa": {"nuviotv", "3", "3", "7", "unsigned", "release"},
		"App.tipa":                           {"app"},
		"Nuvio TV for tvOS":                  {"nuvio", "tv", "for", "tvos"},
		"com.example.app.tvos":               {"com", "example", "app", "tvos"},
	}
	for name, want := range cases {
		if got := Tokens(name); !reflect.DeepEqual(got, want) {
			t.Errorf("Tokens(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestPlatformHint(t *testing.T) {
	cases := []struct {
		texts []string
		want  Platform
	}{
		{[]string{"OrivioTV-V9.Sideload.ipa"}, PlatformUnknown},
		{[]string{"Studios.ipa"}, PlatformUnknown},
		{[]string{"App.tipa"}, PlatformUnknown},
		{[]string{"Provenance-tvOS.ipa"}, PlatformTVOS},
		{[]string{"Swiftfin_tvOS_1.2.ipa"}, PlatformTVOS},
		{[]string{"App-AppleTV.ipa"}, PlatformTVOS},
		{[]string{"App-iOS.ipa"}, PlatformIOS},
		{[]string{"App-iPhoneOS.ipa"}, PlatformIOS},
		{[]string{"App-iOS-tvOS.ipa"}, PlatformUnknown},
		{[]string{"NuvioTVOS", "Nuvio TV for tvOS", "com.pyksel.nuviotvos", "NuvioTV-3.3.7-unsigned-release.ipa"}, PlatformTVOS},
		{nil, PlatformUnknown},
	}
	for _, c := range cases {
		if got := PlatformHint(c.texts...); got != c.want {
			t.Errorf("PlatformHint(%q) = %q, want %q", c.texts, got, c.want)
		}
	}
}

func TestDevicePlatformAndOpposite(t *testing.T) {
	if DevicePlatform("AppleTV") != PlatformTVOS || DevicePlatform("iPhone") != PlatformIOS ||
		DevicePlatform("iPad") != PlatformIOS || DevicePlatform("") != PlatformUnknown {
		t.Fatal("unexpected DevicePlatform result")
	}
	if Opposite(PlatformTVOS) != PlatformIOS || Opposite(PlatformIOS) != PlatformTVOS || Opposite(PlatformUnknown) != PlatformUnknown {
		t.Fatal("unexpected Opposite result")
	}
}

func TestDeriveFilter(t *testing.T) {
	cases := []struct {
		picked string
		others []string
		want   string
	}{
		{"OrivioTV-V9.Sideload.ipa", []string{"OrivioTV-V9.Sideload.ipa", "OrivioTV-V9.Sideloadly.ipa"}, `(?i)(^|[^a-z0-9])sideload([^a-z0-9]|$)`},
		{"OrivioTV-V9.Sideloadly.ipa", []string{"OrivioTV-V9.Sideload.ipa"}, `(?i)(^|[^a-z0-9])sideloadly([^a-z0-9]|$)`},
		{"OrivioTV-0.7.15-tvos-unsigned.ipa", []string{"OrivioTV-0.7.15-sideloadly-unsigned.ipa"}, `(?i)(^|[^a-z0-9])tvos([^a-z0-9]|$)`},
		{"App-tvOS.ipa", []string{"App-iOS.ipa"}, `(?i)(^|[^a-z0-9])tvos([^a-z0-9]|$)`},
		{"NuvioTV-3.3.7-unsigned-release.ipa", nil, `(?i)(^|[^a-z0-9])nuviotv([^a-z0-9]|$)`},
		{"v2.ipa", []string{"App.ipa"}, `(?i)^v[0-9]+(\.[0-9]+)*\.ipa$`},
		{"App.ipa", []string{"App-tvOS.ipa"}, `(?i)^App\.ipa$`},
	}
	for _, c := range cases {
		if got := DeriveFilter(c.picked, c.others); got != c.want {
			t.Errorf("DeriveFilter(%q) = %q, want %q", c.picked, got, c.want)
		}
	}
}

// TestDeriveFilterUnmarkedVariant checks that the filter of an unmarked build
// next to a marked variant keeps matching when the version changes.
func TestDeriveFilterUnmarkedVariant(t *testing.T) {
	cases := []struct {
		picked, other string
		match, skip   []string
	}{
		{"App-1.2.ipa", "App-1.2-iOS.ipa", []string{"App-1.3.ipa", "App-1.2.1.ipa", "app-2.ipa"}, []string{"App-1.3-iOS.ipa", "Other-1.3.ipa"}},
		{"OrivioTV-V9.ipa", "OrivioTV-V9.Sideloadly.ipa", []string{"OrivioTV-V10.ipa"}, []string{"OrivioTV-V10.Sideloadly.ipa"}},
	}
	for _, c := range cases {
		re := regexp.MustCompile(DeriveFilter(c.picked, []string{c.picked, c.other}))
		for _, name := range append([]string{c.picked}, c.match...) {
			if !re.MatchString(name) {
				t.Errorf("filter of %q does not match %q", c.picked, name)
			}
		}
		for _, name := range append([]string{c.other}, c.skip...) {
			if re.MatchString(name) {
				t.Errorf("filter of %q matches %q", c.picked, name)
			}
		}
	}

	// In release v1.3 the filter still selects the unmarked build on an Apple TV.
	f := &githubFeed{repo: "owner/app", releases: []githubRelease{release("v1.3", false, asset(1, "App-1.3.ipa"), asset(2, "App-1.3-iOS.ipa"))}}
	b, err := f.Latest(DeriveFilter("App-1.2.ipa", []string{"App-1.2.ipa", "App-1.2-iOS.ipa"}), false, "AppleTV")
	if err != nil || b.Name != "App-1.3.ipa" {
		t.Fatalf("Latest = %+v, %v; want App-1.3.ipa", b, err)
	}
}

// TestDeriveFilterNumberedVariants checks that variants differing only in
// numbers get filters that tell them apart in the release they come from.
func TestDeriveFilterNumberedVariants(t *testing.T) {
	for _, pair := range [][2]string{
		{"App_tvOS_15.ipa", "App_tvOS_17.ipa"},
		{"Kodi-21.1-tvOS.ipa", "Kodi-20.5-tvOS.ipa"},
	} {
		for i, picked := range pair {
			other := pair[1-i]
			re := regexp.MustCompile(DeriveFilter(picked, pair[:]))
			if !re.MatchString(picked) || re.MatchString(other) {
				t.Errorf("filter %q of %q: match own = %v, match %q = %v", re, picked, re.MatchString(picked), other, re.MatchString(other))
			}
		}
	}

	// Latest finds the picked variant instead of reporting several matches.
	f := &githubFeed{repo: "owner/kodi", releases: []githubRelease{release("v21.1", false, asset(1, "Kodi-21.1-tvOS.ipa"), asset(2, "Kodi-20.5-tvOS.ipa"))}}
	b, err := f.Latest(DeriveFilter("Kodi-20.5-tvOS.ipa", []string{"Kodi-21.1-tvOS.ipa", "Kodi-20.5-tvOS.ipa"}), false, "AppleTV")
	if err != nil || b.Name != "Kodi-20.5-tvOS.ipa" {
		t.Fatalf("Latest = %+v, %v; want Kodi-20.5-tvOS.ipa", b, err)
	}
}

// TestDeriveFilterOrivioHistory checks that filters derived from the newest
// OrivioTVAppleTV release keep selecting the same variant in older releases.
func TestDeriveFilterOrivioHistory(t *testing.T) {
	sideload := regexp.MustCompile(DeriveFilter("OrivioTV-V9.Sideload.ipa", []string{"OrivioTV-V9.Sideloadly.ipa"}))
	sideloadly := regexp.MustCompile(DeriveFilter("OrivioTV-V9.Sideloadly.ipa", []string{"OrivioTV-V9.Sideload.ipa"}))

	cases := []struct {
		name                 string
		sideload, sideloadly bool
	}{
		{"OrivioTV-V9.Sideload.ipa", true, false},
		{"OrivioTV-V9.Sideloadly.ipa", false, true},
		{"OrivioTV-V7.Sideload.ipa", true, false},
		{"OrivioTV-V7.Sideloadly.ipa", false, true},
		{"OrivioTV-V2.Sideloadly.ipa", false, true},
		{"OrivioTV-0.7.15-sideloadly-unsigned.ipa", false, true},
		{"OrivioTV-0.7.15-tvos-unsigned.ipa", false, false},
		{"OrivioTV-V10.SIDELOAD.ipa", true, false},
		{"OrivioTV-V10.SideloadLy.ipa", false, true},
	}
	for _, c := range cases {
		if got := sideload.MatchString(c.name); got != c.sideload {
			t.Errorf("sideload filter on %q = %v, want %v", c.name, got, c.sideload)
		}
		if got := sideloadly.MatchString(c.name); got != c.sideloadly {
			t.Errorf("sideloadly filter on %q = %v, want %v", c.name, got, c.sideloadly)
		}
	}
}

func TestSuggest(t *testing.T) {
	tv := Build{ID: "tv", Platform: PlatformTVOS}
	ios := Build{ID: "ios", Platform: PlatformIOS}
	unknownA := Build{ID: "a"}
	unknownB := Build{ID: "b"}

	cases := []struct {
		name        string
		builds      []Build
		deviceClass string
		want        string
	}{
		{"tvOS among iOS", []Build{ios, tv}, "AppleTV", "tv"},
		{"iOS among tvOS", []Build{ios, tv}, "iPhone", "ios"},
		{"exact beats unknown", []Build{unknownA, tv}, "AppleTV", "tv"},
		{"single compatible", []Build{ios, unknownA}, "AppleTV", "a"},
		{"two variants", []Build{unknownA, unknownB}, "AppleTV", ""},
		{"only other platform", []Build{ios}, "AppleTV", ""},
		{"unknown device single", []Build{unknownA}, "", "a"},
		{"unknown device several", []Build{tv, ios}, "", ""},
		{"no builds", nil, "AppleTV", ""},
	}
	for _, c := range cases {
		if got := Suggest(c.builds, c.deviceClass); got != c.want {
			t.Errorf("%s: Suggest = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestValidateFilterAndFilterMatches(t *testing.T) {
	if err := ValidateFilter(KindGitHub, ""); err == nil {
		t.Error("empty GitHub filter should be invalid")
	}
	if err := ValidateFilter(KindGitHub, "(unclosed"); err == nil {
		t.Error("invalid regexp should be rejected")
	}
	if err := ValidateFilter(KindGitHub, `(?i)tvos`); err != nil {
		t.Errorf("valid regexp rejected: %v", err)
	}
	if err := ValidateFilter(KindAltStore, ""); err == nil {
		t.Error("empty AltStore filter should be invalid")
	}
	if err := ValidateFilter(KindAltStore, "com.example.app"); err != nil {
		t.Errorf("AltStore bundle id rejected: %v", err)
	}
	if err := ValidateFilter("gitlab", "x"); err == nil {
		t.Error("unknown kind should be rejected")
	}

	b := Build{Name: "App-tvOS.ipa", BundleID: "com.example.app"}
	if !FilterMatches(KindGitHub, `(?i)tvos`, b) || FilterMatches(KindGitHub, `ios\.ipa$`, b) || FilterMatches(KindGitHub, "(", b) {
		t.Error("unexpected GitHub FilterMatches result")
	}
	if !FilterMatches(KindAltStore, "com.example.app", b) || FilterMatches(KindAltStore, "com.example", b) {
		t.Error("unexpected AltStore FilterMatches result")
	}
}

func TestFileName(t *testing.T) {
	cases := map[string]string{
		"https://github.com/bobsupra/NuvioTVOS/releases/download/tvos-beta-3.3.7/NuvioTV-3.3.7-unsigned-release.ipa": "NuvioTV-3.3.7-unsigned-release.ipa",
		"https://example.com/dl/App%20TV.ipa?token=1":                                                                "App TV.ipa",
	}
	for u, want := range cases {
		if got := FileName(u); got != want {
			t.Errorf("FileName(%q) = %q, want %q", u, got, want)
		}
	}
}
