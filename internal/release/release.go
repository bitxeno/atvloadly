// Package release checks GitHub for new atvloadly releases, keeps the latest
// result for the web UI and notifies once per new version.
package release

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	atvhttp "github.com/bitxeno/atvloadly/internal/http"
	"github.com/bitxeno/atvloadly/internal/i18n"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/notify"
	"golang.org/x/mod/semver"
)

// Repo is the GitHub repository that publishes atvloadly releases.
const Repo = "bitxeno/atvloadly"

const requestTimeout = 30 * time.Second

// apiBaseURL is the GitHub REST API root; tests point it to a local server.
var apiBaseURL = "https://api.github.com"

// sendNotification is replaced in tests.
var sendNotification = notify.Send

// Info describes the running version and the latest published release.
type Info struct {
	Current     string    `json:"current"`
	Latest      string    `json:"latest"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
	// Available reports whether Latest is newer than Current. It is false for
	// development and nightly builds, whose version is not a release tag.
	Available bool `json:"available"`
}

type githubRelease struct {
	TagName     string    `json:"tag_name"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
}

var (
	mu        sync.RWMutex
	latest    githubRelease
	checked   []func(Info)
	announced []func(Info)
)

// OnCheck registers fn to be called with the result of every successful
// check. The desktop app uses it to show the release in its tray menu.
func OnCheck(fn func(Info)) {
	mu.Lock()
	checked = append(checked, fn)
	mu.Unlock()
}

// OnNewRelease registers fn to be called once per newer release, together
// with the configured notification. The desktop app uses it to show a
// native notification.
func OnNewRelease(fn func(Info)) {
	mu.Lock()
	announced = append(announced, fn)
	mu.Unlock()
}

// Latest returns the result of the last successful check.
func Latest() Info {
	mu.RLock()
	r := latest
	mu.RUnlock()
	return newInfo(app.Version.Version, r)
}

// Check fetches the latest release, stores it and sends a notification the
// first time a newer version is seen.
func Check() error {
	r, err := fetchLatest()
	if err != nil {
		return err
	}
	mu.Lock()
	latest = r
	onCheck, onNew := checked, announced
	mu.Unlock()

	info := newInfo(app.Version.Version, r)
	for _, fn := range onCheck {
		fn(info)
	}
	if !info.Available {
		return nil
	}
	log.Infof("A new atvloadly release is available: %s (running %s)", info.Latest, info.Current)
	announce(info, onNew)
	return nil
}

// announce sends the release notification and calls the OnNewRelease
// handlers unless this version was already announced, which survives
// restarts through the settings file. A failed notification is retried at
// the next check, unless a handler already announced the release. Nothing is
// announced while release notifications are disabled.
func announce(info Info, handlers []func(Info)) {
	if !app.Settings.Update.ReleaseNotify || app.Settings.Update.NotifiedRelease == info.Latest {
		return
	}
	done := false
	if app.Settings.Notification.Enabled {
		title := i18n.LocalizeF("notify.release_title", map[string]any{"version": info.Latest})
		message := i18n.LocalizeF("notify.release_content", map[string]any{"current": info.Current, "url": info.URL})
		if err := sendNotification(title, message); err != nil {
			log.Err(err).Msg("Failed to send the release notification")
		} else {
			done = true
		}
	}
	for _, fn := range handlers {
		fn(info)
		done = true
	}
	if done {
		app.Settings.Update.NotifiedRelease = info.Latest
		app.SaveSettings()
	}
}

func newInfo(current string, r githubRelease) Info {
	info := Info{
		Current:     current,
		Latest:      r.TagName,
		URL:         r.HTMLURL,
		PublishedAt: r.PublishedAt,
	}
	cur, next := canonical(current), canonical(r.TagName)
	info.Available = cur != "" && next != "" && semver.Compare(next, cur) > 0
	return info
}

// canonical returns v as a "vX.Y.Z" semantic version, or "" when it is not
// one. Desktop builds carry the version without the "v" prefix.
func canonical(v string) string {
	v = strings.TrimSpace(v)
	if v != "" && !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if !semver.IsValid(v) {
		return ""
	}
	return v
}

// fetchLatest returns the latest stable release; GitHub excludes drafts and
// pre-releases from this endpoint.
func fetchLatest() (githubRelease, error) {
	var r githubRelease
	req := atvhttp.NewClient().SetTimeout(requestTimeout).R().
		SetHeader("Accept", "application/vnd.github+json").
		SetHeader("X-GitHub-Api-Version", "2022-11-28").
		SetResult(&r)
	if token := app.GetGitHubToken(); token != "" {
		req.SetHeader("Authorization", "Bearer "+token)
	}
	resp, err := req.Get(fmt.Sprintf("%s/repos/%s/releases/latest", apiBaseURL, Repo))
	if err != nil {
		return r, fmt.Errorf("failed to fetch the latest atvloadly release: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return r, fmt.Errorf("failed to fetch the latest atvloadly release: HTTP %d", resp.StatusCode())
	}
	if r.TagName == "" {
		return r, fmt.Errorf("the latest atvloadly release has no tag")
	}
	return r, nil
}
