// Package source reads the builds published by a GitHub repository's releases
// or by an AltStore source, so installed apps can be installed from and track
// updates of the IPA they were built from.
package source

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"time"

	"github.com/go-resty/resty/v2"

	atvhttp "github.com/bitxeno/atvloadly/internal/http"
)

const (
	KindGitHub   = "github"
	KindAltStore = "altstore"
)

// Platform is a hint derived from names; never used alone to install.
type Platform string

const (
	PlatformUnknown Platform = ""
	PlatformTVOS    Platform = "tvos"
	PlatformIOS     Platform = "ios"
)

const requestTimeout = 30 * time.Second

// Build is one installable IPA published by a source.
type Build struct {
	ID          string    `json:"id"`           // GitHub: asset id; AltStore: stable hash of the version
	Name        string    `json:"name"`         // GitHub: asset file name; AltStore: app name
	Version     string    `json:"version"`      // GitHub: release tag; AltStore: version (and build)
	Date        time.Time `json:"date"`         // GitHub: asset creation date; AltStore: version date
	Size        int64     `json:"size"`         // bytes
	DownloadURL string    `json:"download_url"` // IPA download URL
	PageURL     string    `json:"page_url"`     // GitHub: release page
	IconURL     string    `json:"icon_url"`     // AltStore: app icon
	BundleID    string    `json:"bundle_id"`    // AltStore: bundle identifier
	Prerelease  bool      `json:"prerelease"`   // GitHub: release is a pre-release
	Platform    Platform  `json:"platform"`     // name-based hint
	Filter      string    `json:"filter"`       // filter that keeps tracking this build in later releases
}

// Preview is what the user picks from when installing or linking.
type Preview struct {
	Kind        string  `json:"kind"`
	URL         string  `json:"url"`          // normalized: GitHub "owner/repo"; AltStore the source URL
	Title       string  `json:"title"`        // GitHub: release name or tag; AltStore: source name
	PageURL     string  `json:"page_url"`     // GitHub: release page; AltStore: source URL
	Builds      []Build `json:"builds"`       // builds the user can pick from
	SuggestedID string  `json:"suggested_id"` // "" when the user must choose
}

// Feed is one fetched source (one network request).
type Feed interface {
	// Preview lists the builds the user can pick for deviceClass.
	Preview(deviceClass string, prerelease bool) (*Preview, error)
	// Latest returns the newest build tracked by filter.
	Latest(filter string, prerelease bool, deviceClass string) (Build, error)
	// Find returns the build with id, searching everything that was fetched.
	Find(id string) (Build, error)
}

var (
	// ErrNotFound reports a source that does not exist or is private.
	ErrNotFound = errors.New("not found")
	// ErrNoMatch reports that no build matches; it is wrapped with details.
	ErrNoMatch = errors.New("no matching build")
	// ErrAmbiguous reports that several builds match and the user must choose.
	ErrAmbiguous = errors.New("several builds match")
)

// RateLimitError reports the GitHub unauthenticated quota (60 req/h) is
// exhausted until Reset.
type RateLimitError struct {
	Reset time.Time
}

func (e *RateLimitError) Error() string {
	return "GitHub API rate limit exceeded, retry after " + e.Reset.Local().Format("15:04")
}

// Normalize validates and normalizes a user-supplied source location.
func Normalize(kind, location string) (string, error) {
	switch kind {
	case KindGitHub:
		return ParseRepo(location)
	case KindAltStore:
		return normalizeAltStoreURL(location)
	default:
		return "", fmt.Errorf("unknown source kind %q", kind)
	}
}

// Fetch downloads the source with exactly one HTTP request. location must be
// normalized.
func Fetch(kind, location string) (Feed, error) {
	switch kind {
	case KindGitHub:
		return fetchGitHub(location)
	case KindAltStore:
		return fetchAltStore(location)
	default:
		return nil, fmt.Errorf("unknown source kind %q", kind)
	}
}

// ValidateFilter checks a stored filter for kind: a GitHub filter is a
// regular expression on the asset name, an AltStore filter a bundle identifier.
func ValidateFilter(kind, filter string) error {
	if filter == "" {
		return errors.New("the filter is empty")
	}
	switch kind {
	case KindGitHub:
		if _, err := regexp.Compile(filter); err != nil {
			return fmt.Errorf("invalid filter: %w", err)
		}
		return nil
	case KindAltStore:
		return nil
	default:
		return fmt.Errorf("unknown source kind %q", kind)
	}
}

// FilterMatches reports whether build b is selected by filter.
func FilterMatches(kind, filter string, b Build) bool {
	switch kind {
	case KindGitHub:
		re, err := regexp.Compile(filter)
		return err == nil && re.MatchString(b.Name)
	case KindAltStore:
		return filter != "" && filter == b.BundleID
	default:
		return false
	}
}

// FileName returns the file name of a download URL.
func FileName(downloadURL string) string {
	p := downloadURL
	if u, err := url.Parse(downloadURL); err == nil {
		p = u.Path
	}
	return path.Base(p)
}

func newClient() *resty.Client {
	return atvhttp.NewClient().SetTimeout(requestTimeout)
}
