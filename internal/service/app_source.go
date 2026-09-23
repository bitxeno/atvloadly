package service

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bitxeno/atvloadly/internal/db"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/source"
)

var (
	// checkMu serializes update checks so concurrent runs neither race on the
	// check columns nor repeat the same requests.
	checkMu sync.Mutex
	// fetchSource downloads a source; tests replace it.
	fetchSource = source.Fetch
)

// SourceInput links an installed app to a source.
type SourceInput struct {
	Kind             string `json:"kind"`
	URL              string `json:"url"`
	Filter           string `json:"filter"`
	Prerelease       bool   `json:"prerelease"`
	AutoUpdate       bool   `json:"auto_update"`
	InstalledBuildID string `json:"installed_build_id"` // optional: "the installed version is already this build"
}

// SourceNotice is a change found by an update check that the user should know.
type SourceNotice struct {
	AppName string // custom name or ipa name
	Version string
	Error   string // empty for "update available"
}

// SourceCheckResult is what an update check found.
type SourceCheckResult struct {
	Updates []model.InstalledApp // ApplyBuild target states for every tracked app that has a newer build
	Notices []SourceNotice
}

// sourceColumns returns every source column of s, zero values included.
func sourceColumns(s model.AppSource) map[string]any {
	return map[string]any{
		"source_kind":            s.Kind,
		"source_url":             s.URL,
		"source_filter":          s.Filter,
		"source_prerelease":      s.Prerelease,
		"source_auto_update":     s.AutoUpdate,
		"source_build_id":        s.BuildID,
		"source_version":         s.Version,
		"source_build_name":      s.BuildName,
		"source_build_date":      s.BuildDate,
		"source_checked_at":      s.CheckedAt,
		"source_check_error":     s.CheckError,
		"source_latest_build_id": s.LatestBuildID,
		"source_latest_version":  s.LatestVersion,
		"source_failed_build_id": s.FailedBuildID,
	}
}

// checkColumns returns the columns written by an update check.
func checkColumns(s model.AppSource) map[string]any {
	return map[string]any{
		"source_checked_at":      s.CheckedAt,
		"source_check_error":     s.CheckError,
		"source_latest_build_id": s.LatestBuildID,
		"source_latest_version":  s.LatestVersion,
	}
}

// isMatchError reports whether err means the source has no single build to
// install, which needs the user's attention.
func isMatchError(err error) bool {
	return errors.Is(err, source.ErrNoMatch) || errors.Is(err, source.ErrAmbiguous)
}

// checkBundle rejects an AltStore filter selecting another app than the
// installed one.
func checkBundle(kind, filter string, app *model.InstalledApp) error {
	if kind == source.KindAltStore && app.BundleIdentifier != "" && filter != app.BundleIdentifier {
		return fmt.Errorf("the source app %s does not match the installed bundle %s", filter, app.BundleIdentifier)
	}
	return nil
}

// PreviewSource lists the builds of a source the user can install on a device
// of deviceClass.
func PreviewSource(kind, location, deviceClass string, prerelease bool) (*source.Preview, error) {
	location, err := source.Normalize(kind, location)
	if err != nil {
		return nil, err
	}
	feed, err := fetchSource(kind, location)
	if err != nil {
		return nil, err
	}
	return feed.Preview(deviceClass, prerelease)
}

// EvaluateSource returns s with the check fields updated from a Latest() result.
func EvaluateSource(s model.AppSource, b source.Build, err error, now time.Time) model.AppSource {
	s.CheckedAt = &now
	switch {
	case isMatchError(err):
		s.CheckError = err.Error()
		s.LatestBuildID, s.LatestVersion = "", ""
	case err != nil:
		// The source could not be read: keep what the last check found.
		s.CheckError = err.Error()
	case b.ID == s.BuildID || (s.BuildDate != nil && !b.Date.IsZero() && !b.Date.After(*s.BuildDate)):
		// Installed, or older than the installed build (never downgrade).
		s.CheckError, s.LatestBuildID, s.LatestVersion = "", "", ""
	default:
		s.CheckError, s.LatestBuildID, s.LatestVersion = "", b.ID, b.Version
	}
	return s
}

// installedBuild returns s recording b as the installed build, with the
// results of previous checks cleared.
func installedBuild(s model.AppSource, b source.Build) model.AppSource {
	s.BuildID = b.ID
	s.Version = b.Version
	s.BuildName = b.Name
	if s.Kind == source.KindAltStore {
		s.BuildName = source.FileName(b.DownloadURL)
	}
	s.BuildDate = nil
	if !b.Date.IsZero() {
		date := b.Date
		s.BuildDate = &date
	}
	s.LatestBuildID, s.LatestVersion, s.CheckError, s.FailedBuildID = "", "", "", ""
	return s
}

// ApplyBuild returns app as it should be after installing b: the IPA is
// downloaded from b and b is recorded as the installed build.
func ApplyBuild(app model.InstalledApp, b source.Build) model.InstalledApp {
	app.IpaPath = b.DownloadURL
	app.Source = installedBuild(app.Source, b)
	return app
}

// ResolveSourceInstall resolves the build the user picked for a first install
// from a source: v.Source holds the link settings and the picked BuildID. The
// download URL always comes from the source, never from the client.
func ResolveSourceInstall(v *model.InstalledApp) error {
	in := v.Source
	location, err := source.Normalize(in.Kind, in.URL)
	if err != nil {
		return err
	}
	if err := source.ValidateFilter(in.Kind, in.Filter); err != nil {
		return err
	}
	feed, err := fetchSource(in.Kind, location)
	if err != nil {
		return err
	}
	b, err := feed.Find(in.BuildID)
	if err != nil {
		return err
	}
	if !source.FilterMatches(in.Kind, in.Filter, b) {
		return fmt.Errorf("the filter does not match %s", b.Name)
	}

	v.Source = model.AppSource{
		Kind:       in.Kind,
		URL:        location,
		Filter:     in.Filter,
		Prerelease: in.Prerelease,
		AutoUpdate: in.AutoUpdate,
	}
	*v = ApplyBuild(*v, b)
	return nil
}

// CheckSourceUpdates checks every tracked app in apps, persists the check
// fields and reports what changed. Each distinct source is fetched once.
func CheckSourceUpdates(apps []model.InstalledApp) SourceCheckResult {
	checkMu.Lock()
	defer checkMu.Unlock()

	type fetched struct {
		feed source.Feed
		err  error
	}
	feeds := map[string]fetched{}
	var rateLimit *source.RateLimitError
	var res SourceCheckResult
	now := time.Now()
	for _, app := range apps {
		s := app.Source
		if !s.Tracked() {
			continue
		}

		key := s.Kind + "\n" + strings.ToLower(s.URL)
		f, ok := feeds[key]
		if !ok {
			if s.Kind == source.KindGitHub && rateLimit != nil {
				// The GitHub quota is exhausted for the rest of this run.
				f.err = rateLimit
			} else {
				f.feed, f.err = fetchSource(s.Kind, s.URL)
				if errors.As(f.err, &rateLimit) {
					log.Warnf("Skip checking updates from GitHub: %s", f.err)
				}
			}
			feeds[key] = f
		}
		var limited *source.RateLimitError
		if errors.As(f.err, &limited) {
			// Nothing is known about this source: keep the last check.
			continue
		}

		var b source.Build
		err := f.err
		if err == nil {
			b, err = f.feed.Latest(s.Filter, s.Prerelease, app.DeviceClass)
		}
		if err != nil && !isMatchError(err) {
			log.Err(err).Msgf("Check updates of %s from %s failed", app.DisplayName(), s.URL)
		}

		next := EvaluateSource(s, b, err, now)
		if result := db.Store().Model(&model.InstalledApp{}).Where("id = ?", app.ID).Updates(checkColumns(next)); result.Error != nil {
			log.Err(result.Error).Msgf("Save update check of %s failed", app.DisplayName())
		}
		if err == nil && next.UpdateAvailable() {
			app.Source = next
			res.Updates = append(res.Updates, ApplyBuild(app, b))
		}

		// Notify each new finding once; fetch errors are only logged.
		switch {
		case isMatchError(err) && (next.CheckError != s.CheckError || next.LatestBuildID != s.LatestBuildID):
			res.Notices = append(res.Notices, SourceNotice{AppName: app.DisplayName(), Error: next.CheckError})
		case err == nil && next.UpdateAvailable() && !s.AutoUpdate && next.LatestBuildID != s.LatestBuildID:
			res.Notices = append(res.Notices, SourceNotice{AppName: app.DisplayName(), Version: next.LatestVersion})
		}
	}
	return res
}

// LinkSource links app id to a source (or changes its settings), then checks
// it for updates.
func LinkSource(id uint, in SourceInput) (*model.InstalledApp, error) {
	app, err := GetApp(id)
	if err != nil {
		return nil, err
	}
	location, err := source.Normalize(in.Kind, in.URL)
	if err != nil {
		return nil, err
	}
	if err := source.ValidateFilter(in.Kind, in.Filter); err != nil {
		return nil, err
	}
	if err := checkBundle(in.Kind, in.Filter, app); err != nil {
		return nil, err
	}

	s := app.Source
	if s.Kind != in.Kind || !strings.EqualFold(s.URL, location) {
		// What was recorded about another source does not apply.
		s = model.AppSource{}
	}
	s.Kind, s.URL, s.Filter, s.Prerelease, s.AutoUpdate = in.Kind, location, in.Filter, in.Prerelease, in.AutoUpdate

	if in.InstalledBuildID != "" {
		feed, err := fetchSource(in.Kind, location)
		if err != nil {
			return nil, err
		}
		b, err := feed.Find(in.InstalledBuildID)
		if err != nil {
			return nil, err
		}
		if !source.FilterMatches(in.Kind, in.Filter, b) {
			return nil, fmt.Errorf("the filter does not match %s", b.Name)
		}
		s = installedBuild(s, b)
	}

	if result := db.Store().Model(app).Updates(sourceColumns(s)); result.Error != nil {
		return nil, result.Error
	}
	app.Source = s
	CheckSourceUpdates([]model.InstalledApp{*app})

	return GetApp(id)
}

// UntrackSource unlinks app id from its source.
func UntrackSource(id uint) error {
	app, err := GetApp(id)
	if err != nil {
		return err
	}
	return db.Store().Model(app).Updates(sourceColumns(model.AppSource{})).Error
}

// PrepareSourceUpdate returns app id as it should be after installing the
// build buildID of its source, or the latest build when buildID is empty. A
// non-empty filter replaces the stored one once the update is installed, so a
// failed switch to another variant keeps tracking the installed one.
func PrepareSourceUpdate(id uint, buildID, filter string) (model.InstalledApp, error) {
	app, err := GetApp(id)
	if err != nil {
		return model.InstalledApp{}, err
	}
	s := app.Source
	if !s.Tracked() {
		return model.InstalledApp{}, errors.New("the app does not track a source")
	}
	if filter == "" {
		filter = s.Filter
	} else {
		if err := source.ValidateFilter(s.Kind, filter); err != nil {
			return model.InstalledApp{}, err
		}
		if err := checkBundle(s.Kind, filter, app); err != nil {
			return model.InstalledApp{}, err
		}
	}

	feed, err := fetchSource(s.Kind, s.URL)
	if err != nil {
		return model.InstalledApp{}, err
	}
	var b source.Build
	if buildID == "" {
		b, err = feed.Latest(filter, s.Prerelease, app.DeviceClass)
	} else {
		b, err = feed.Find(buildID)
	}
	if err != nil {
		return model.InstalledApp{}, err
	}
	if !source.FilterMatches(s.Kind, filter, b) {
		return model.InstalledApp{}, fmt.Errorf("the filter does not match %s", b.Name)
	}

	app.Source.Filter = filter
	return ApplyBuild(*app, b), nil
}

// MarkSourceUpdateFailed records that installing build buildID of app id
// failed, so it is not retried automatically.
func MarkSourceUpdateFailed(id uint, buildID, msg string) error {
	updateData := map[string]any{
		"source_failed_build_id": buildID,
		"source_check_error":     "update failed: " + msg,
	}
	return db.Store().Model(&model.InstalledApp{}).Where("id = ?", id).Updates(updateData).Error
}

// GetTrackedAppList returns the apps linked to a source.
func GetTrackedAppList() ([]model.InstalledApp, error) {
	var apps []model.InstalledApp
	if result := db.Store().Where("source_kind <> ?", "").Order("created_at desc").Find(&apps); result.Error != nil {
		return nil, result.Error
	}

	return apps, nil
}
