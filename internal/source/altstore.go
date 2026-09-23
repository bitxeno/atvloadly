package source

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// altStoreDateLayouts are the date formats found in AltStore sources.
var altStoreDateLayouts = []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"}

// altStoreSource is an AltStore / SideStore / Feather source JSON.
type altStoreSource struct {
	Name       string        `json:"name"`
	Identifier string        `json:"identifier"`
	Apps       []altStoreApp `json:"apps"`
}

type altStoreApp struct {
	Name                 string            `json:"name"`
	BundleIdentifier     string            `json:"bundleIdentifier"`
	DeveloperName        string            `json:"developerName"`
	Subtitle             string            `json:"subtitle"`
	IconURL              string            `json:"iconURL"`
	LocalizedDescription string            `json:"localizedDescription"`
	Version              string            `json:"version"`
	VersionDate          string            `json:"versionDate"`
	DownloadURL          string            `json:"downloadURL"`
	Size                 flexSize          `json:"size"`
	Versions             []altStoreVersion `json:"versions"`
}

type altStoreVersion struct {
	Version      string   `json:"version"`
	BuildVersion string   `json:"buildVersion"`
	Date         string   `json:"date"`
	DownloadURL  string   `json:"downloadURL"`
	Size         flexSize `json:"size"`
}

// flexSize decodes a size given as a number or a numeric string; anything
// else decodes to 0.
type flexSize int64

func (s *flexSize) UnmarshalJSON(data []byte) error {
	if n, err := strconv.ParseFloat(strings.Trim(string(data), `"`), 64); err == nil {
		*s = flexSize(n)
	}
	return nil
}

// altStoreEntry is an app of the source with its downloadable versions,
// newest first.
type altStoreEntry struct {
	bundleID string
	builds   []Build
}

type altStoreFeed struct {
	url     string
	name    string
	entries []altStoreEntry
}

func normalizeAltStoreURL(location string) (string, error) {
	s := strings.TrimSpace(location)
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("invalid source URL %q, expected an http(s) URL", location)
	}
	return s, nil
}

// fetchAltStore downloads and decodes the source JSON at u.
func fetchAltStore(u string) (Feed, error) {
	resp, err := newClient().R().SetHeader("Accept", "application/json").Get(u)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch source: %w", err)
	}
	switch resp.StatusCode() {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, fmt.Errorf("source %s %w", u, ErrNotFound)
	default:
		return nil, fmt.Errorf("source returned %d", resp.StatusCode())
	}

	var src altStoreSource
	if err := json.Unmarshal(bytes.TrimPrefix(resp.Body(), []byte("\xef\xbb\xbf")), &src); err != nil {
		return nil, fmt.Errorf("invalid AltStore source: %w", err)
	}

	f := &altStoreFeed{url: u, name: src.Name}
	for _, app := range src.Apps {
		f.entries = append(f.entries, altStoreEntry{bundleID: app.BundleIdentifier, builds: appBuilds(app)})
	}
	return f, nil
}

// appBuilds returns the downloadable versions of app in source order (newest
// first). Legacy apps without versions describe a single version themselves.
func appBuilds(app altStoreApp) []Build {
	versions := app.Versions
	if len(versions) == 0 {
		versions = []altStoreVersion{{
			Version:     app.Version,
			Date:        app.VersionDate,
			DownloadURL: app.DownloadURL,
			Size:        app.Size,
		}}
	}

	var builds []Build
	for _, v := range versions {
		// Only http(s) downloads: anything else would be read as a server path.
		if u, err := url.Parse(v.DownloadURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			continue
		}
		version := v.Version
		if v.BuildVersion != "" && v.BuildVersion != v.Version {
			version += " (" + v.BuildVersion + ")"
		}
		sum := sha256.Sum256([]byte(strings.Join([]string{app.BundleIdentifier, v.Version, v.BuildVersion, v.Date, v.DownloadURL}, "\n")))
		builds = append(builds, Build{
			ID:          hex.EncodeToString(sum[:])[:16],
			Name:        app.Name,
			Version:     version,
			Date:        parseAltStoreDate(v.Date),
			Size:        int64(v.Size),
			DownloadURL: v.DownloadURL,
			IconURL:     app.IconURL,
			BundleID:    app.BundleIdentifier,
			Platform:    PlatformHint(app.Name, app.Subtitle, app.BundleIdentifier, FileName(v.DownloadURL)),
			Filter:      app.BundleIdentifier,
		})
	}
	return builds
}

// parseAltStoreDate parses a version date, returning the zero time when it is
// missing or has an unknown format.
func parseAltStoreDate(s string) time.Time {
	for _, layout := range altStoreDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// Preview returns the newest version of every app of the source.
func (f *altStoreFeed) Preview(deviceClass string, _ bool) (*Preview, error) {
	var builds []Build
	for _, e := range f.entries {
		if len(e.builds) > 0 {
			builds = append(builds, e.builds[0])
		}
	}
	if len(builds) == 0 {
		return nil, fmt.Errorf("%w: the source has no downloadable app", ErrNoMatch)
	}

	return &Preview{
		Kind:        KindAltStore,
		URL:         f.url,
		Title:       f.name,
		PageURL:     f.url,
		Builds:      builds,
		SuggestedID: Suggest(builds, deviceClass),
	}, nil
}

// Latest returns the newest version of the app whose bundle identifier is
// filter. When several apps of the source use it (the iOS and tvOS builds of
// one app), the platform of the device picks one.
func (f *altStoreFeed) Latest(filter string, _ bool, deviceClass string) (Build, error) {
	var matches []altStoreEntry
	for _, e := range f.entries {
		if e.bundleID == filter {
			matches = append(matches, e)
		}
	}
	if len(matches) > 1 {
		want := DevicePlatform(deviceClass)
		var compatibles, exact []altStoreEntry
		for _, e := range matches {
			if len(e.builds) == 0 || !compatible(e.builds[0].Platform, want) {
				continue
			}
			compatibles = append(compatibles, e)
			if want != PlatformUnknown && e.builds[0].Platform == want {
				exact = append(exact, e)
			}
		}
		if len(exact) > 0 {
			matches = exact
		} else if len(compatibles) > 0 {
			matches = compatibles
		}
	}
	switch {
	case len(matches) == 0:
		return Build{}, fmt.Errorf("%w: app %s not found in the source", ErrNoMatch, filter)
	case len(matches) > 1:
		return Build{}, fmt.Errorf("%w: %d apps in the source use %s", ErrAmbiguous, len(matches), filter)
	case len(matches[0].builds) == 0:
		return Build{}, fmt.Errorf("%w: app %s has no downloadable version", ErrNoMatch, filter)
	default:
		return matches[0].builds[0], nil
	}
}

// Find returns the version with id of any app of the source.
func (f *altStoreFeed) Find(id string) (Build, error) {
	for _, e := range f.entries {
		for _, b := range e.builds {
			if b.ID == id {
				return b, nil
			}
		}
	}
	return Build{}, fmt.Errorf("%w: version %s not found in the source", ErrNoMatch, id)
}
