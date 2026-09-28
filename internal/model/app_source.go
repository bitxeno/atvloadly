package model

import "time"

// AppSource links an installed app to the GitHub repository or AltStore source
// that publishes its IPA. An empty Kind means the app is not tracked.
type AppSource struct {
	Kind       string `json:"kind"`        // "github", "altstore" or ""
	URL        string `json:"url"`         // GitHub "owner/repo"; AltStore source URL
	Filter     string `json:"filter"`      // GitHub asset-name regexp; AltStore bundle identifier
	Prerelease bool   `json:"prerelease"`  // GitHub only: also consider pre-releases
	AutoUpdate bool   `json:"auto_update"` // install newer builds during the scheduled refresh window

	// Installed build, written when an install from the source succeeds.
	BuildID   string     `json:"build_id"`
	Version   string     `json:"version"`
	BuildName string     `json:"build_name"`
	BuildDate *time.Time `json:"build_date"`

	// Result of the last update check (server-owned).
	CheckedAt     *time.Time `json:"checked_at"`
	CheckError    string     `json:"check_error"`
	LatestBuildID string     `json:"latest_build_id"` // newer build available, "" when up to date or unresolved
	LatestVersion string     `json:"latest_version"`
	FailedBuildID string     `json:"failed_build_id"` // auto-update of this build failed; not retried automatically
}

// Tracked reports whether the app is linked to a source.
func (s AppSource) Tracked() bool { return s.Kind != "" }

// UpdateAvailable reports whether the last check found a newer build.
func (s AppSource) UpdateAvailable() bool { return s.LatestBuildID != "" }
