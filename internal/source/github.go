package source

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// apiBaseURL is the GitHub REST API root; tests point it to a local server.
var apiBaseURL = "https://api.github.com"

var (
	regGitHubOwner = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)
	regGitHubRepo  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
)

type githubRelease struct {
	ID          int64         `json:"id"`
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	HTMLURL     string        `json:"html_url"`
	PublishedAt time.Time     `json:"published_at"`
	Assets      []githubAsset `json:"assets"`
}

type githubAsset struct {
	ID                 int64     `json:"id"`
	Name               string    `json:"name"`
	State              string    `json:"state"`
	Size               int64     `json:"size"`
	CreatedAt          time.Time `json:"created_at"`
	BrowserDownloadURL string    `json:"browser_download_url"`
}

// githubFeed is the list of the latest releases of a repository.
type githubFeed struct {
	repo     string
	releases []githubRelease
}

// ParseRepo returns "owner/repo" from a repository name or GitHub URL such as
// https://github.com/owner/repo/releases/tag/v1.0.
func ParseRepo(input string) (string, error) {
	s := strings.TrimSpace(input)
	hasScheme := false
	if i := strings.Index(s, "://"); i >= 0 {
		if scheme := strings.ToLower(s[:i]); scheme != "http" && scheme != "https" {
			return "", fmt.Errorf("%q is not a GitHub repository URL", input)
		}
		s = s[i+len("://"):]
		hasScheme = true
	}
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}

	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '/' })
	if len(parts) > 0 {
		host := strings.ToLower(parts[0])
		if host == "github.com" || host == "www.github.com" {
			parts = parts[1:]
		} else if hasScheme || strings.Contains(host, ".") {
			return "", fmt.Errorf("%q is not a GitHub repository URL", input)
		}
	}
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid GitHub repository %q, expected owner/repo", input)
	}

	owner, repo := parts[0], strings.TrimSuffix(parts[1], ".git")
	if !regGitHubOwner.MatchString(owner) || !regGitHubRepo.MatchString(repo) || repo == "." || repo == ".." {
		return "", fmt.Errorf("invalid GitHub repository %q, expected owner/repo", input)
	}
	return owner + "/" + repo, nil
}

// fetchGitHub lists the latest releases of repo ("owner/repo").
func fetchGitHub(repo string) (Feed, error) {
	resp, err := newClient().R().
		SetHeader("Accept", "application/vnd.github+json").
		SetHeader("X-GitHub-Api-Version", "2022-11-28").
		Get(fmt.Sprintf("%s/repos/%s/releases?per_page=30", apiBaseURL, repo))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GitHub releases: %w", err)
	}

	switch resp.StatusCode() {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, fmt.Errorf("repository %s %w or private", repo, ErrNotFound)
	default:
		if resp.StatusCode() == http.StatusForbidden || resp.StatusCode() == http.StatusTooManyRequests {
			if err := rateLimitError(resp.Header(), time.Now()); err != nil {
				return nil, err
			}
		}
		var body struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(resp.Body(), &body)
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode(), body.Message)
	}

	var releases []githubRelease
	if err := json.Unmarshal(resp.Body(), &releases); err != nil {
		return nil, fmt.Errorf("invalid GitHub releases response: %w", err)
	}
	return &githubFeed{repo: repo, releases: releases}, nil
}

// rateLimitError returns the rate limit reported by the headers of a 403 or
// 429 response, or nil when the response is not about the rate limit.
func rateLimitError(h http.Header, now time.Time) *RateLimitError {
	if h.Get("X-RateLimit-Remaining") == "0" {
		reset := now.Add(time.Hour)
		if sec, err := strconv.ParseInt(h.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			reset = time.Unix(sec, 0)
		}
		return &RateLimitError{Reset: reset}
	}
	if sec, err := strconv.Atoi(h.Get("Retry-After")); err == nil {
		return &RateLimitError{Reset: now.Add(time.Duration(sec) * time.Second)}
	}
	return nil
}

// eligible reports whether release r is considered for installs.
func eligible(r githubRelease, prerelease bool) bool {
	return !r.Draft && (prerelease || !r.Prerelease)
}

// releaseBuilds returns the uploaded IPA assets of release r.
func releaseBuilds(r githubRelease) []Build {
	var assets []githubAsset
	var names []string
	for _, a := range r.Assets {
		if (a.State == "" || a.State == "uploaded") && IsIPAName(a.Name) {
			assets = append(assets, a)
			names = append(names, a.Name)
		}
	}

	builds := make([]Build, 0, len(assets))
	for _, a := range assets {
		builds = append(builds, Build{
			ID:          strconv.FormatInt(a.ID, 10),
			Name:        a.Name,
			Version:     r.TagName,
			Date:        a.CreatedAt,
			Size:        a.Size,
			DownloadURL: a.BrowserDownloadURL,
			PageURL:     r.HTMLURL,
			Prerelease:  r.Prerelease,
			Platform:    PlatformHint(a.Name),
			Filter:      DeriveFilter(a.Name, names),
		})
	}
	return builds
}

// Preview returns every IPA of the newest release that has one for the device.
func (f *githubFeed) Preview(deviceClass string, prerelease bool) (*Preview, error) {
	want := DevicePlatform(deviceClass)
	for _, r := range f.releases {
		if !eligible(r, prerelease) {
			continue
		}

		var compatibles, others []Build
		for _, b := range releaseBuilds(r) {
			if compatible(b.Platform, want) {
				compatibles = append(compatibles, b)
			} else {
				others = append(others, b)
			}
		}
		if len(compatibles) == 0 {
			continue
		}

		builds := append(compatibles, others...)
		title := r.Name
		if title == "" {
			title = r.TagName
		}
		return &Preview{
			Kind:        KindGitHub,
			URL:         f.repo,
			Title:       title,
			PageURL:     r.HTMLURL,
			Builds:      builds,
			SuggestedID: Suggest(builds, deviceClass),
		}, nil
	}
	return nil, fmt.Errorf("%w: no release with an IPA for this device", ErrNoMatch)
}

// Latest returns the IPA matching filter in the newest release that has an
// IPA for the device. It never guesses: a release whose IPAs were renamed or
// that has several matches is reported instead of skipped.
func (f *githubFeed) Latest(filter string, prerelease bool, deviceClass string) (Build, error) {
	re, err := regexp.Compile(filter)
	if err != nil {
		return Build{}, fmt.Errorf("invalid filter: %w", err)
	}

	want := DevicePlatform(deviceClass)
	for _, r := range f.releases {
		if !eligible(r, prerelease) {
			continue
		}

		var compatibles, matches []Build
		for _, b := range releaseBuilds(r) {
			if compatible(b.Platform, want) {
				compatibles = append(compatibles, b)
				if re.MatchString(b.Name) {
					matches = append(matches, b)
				}
			}
		}
		if len(compatibles) == 0 {
			// Only IPAs for the other platform (or none) in this release.
			continue
		}

		if len(matches) > 1 && want != PlatformUnknown {
			var exact []Build
			for _, b := range matches {
				if b.Platform == want {
					exact = append(exact, b)
				}
			}
			if len(exact) > 0 {
				matches = exact
			}
		}

		switch len(matches) {
		case 1:
			return matches[0], nil
		case 0:
			return Build{}, fmt.Errorf("%w: no IPA in %s matches %s", ErrNoMatch, r.TagName, filter)
		default:
			return Build{}, fmt.Errorf("%w: %d IPAs in %s match %s; choose one", ErrAmbiguous, len(matches), r.TagName, filter)
		}
	}
	return Build{}, fmt.Errorf("%w: no release with an IPA for this device", ErrNoMatch)
}

// Find returns the IPA asset with id from any fetched release.
func (f *githubFeed) Find(id string) (Build, error) {
	for _, r := range f.releases {
		for _, b := range releaseBuilds(r) {
			if b.ID == id {
				return b, nil
			}
		}
	}
	return Build{}, fmt.Errorf("%w: IPA %s not found in the latest releases of %s", ErrNoMatch, id, f.repo)
}
