package main

import (
	"errors"

	"github.com/bitxeno/atvloadly/internal/i18n"
	logx "github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/release"
	"github.com/egoist/mygo"
)

// releaseNotificationID identifies the new-release notification, also when
// its click launches the app again.
const releaseNotificationID = "atvloadly-release"

// latestReleaseURL is the page the notification opens: the release it
// announced, or a newer one by the time it is clicked.
const latestReleaseURL = "https://github.com/" + release.Repo + "/releases/latest"

// watchReleases shows the service's release checks in the app: refreshMenu
// is called on the main thread after every check, and a newer release is
// announced with a native notification. Call it before App.Run, so that the
// click on a notification that launched the app is received.
func watchReleases(refreshMenu func()) {
	release.OnCheck(func(release.Info) {
		mygo.App.WhenReady(refreshMenu)
	})
	release.OnNewRelease(func(info release.Info) {
		mygo.App.WhenReady(func() {
			// Show waits for the permission prompt of the first
			// notification, which must not block the main thread.
			go showReleaseNotification(info)
		})
	})
	mygo.App.OnNotificationClick(func(id string) {
		if id == releaseNotificationID {
			openBrowser(latestReleaseURL)
		}
	})
}

func showReleaseNotification(info release.Info) {
	if !mygo.NotificationsSupported() {
		return
	}
	n := mygo.NewNotification(mygo.NotificationOptions{
		ID:    releaseNotificationID,
		Title: i18n.LocalizeF("notify.release_title", map[string]any{"version": info.Latest}),
		Body:  i18n.LocalizeF("tray.update_notification", map[string]any{"current": info.Current}),
	})
	if err := n.Show(); err != nil && !errors.Is(err, mygo.ErrNotificationsDenied) {
		logx.Err(err).Msg("Failed to show the release notification")
	}
}

// updateMenuItems returns the tray menu items offering a newer release, or
// none.
func updateMenuItems() []*mygo.MenuItem {
	info := release.Latest()
	if !info.Available {
		return nil
	}
	return []*mygo.MenuItem{
		{
			Label: i18n.LocalizeF("tray.update_available", map[string]any{"version": info.Latest}),
			Click: func(*mygo.MenuItem, *mygo.Window) { openBrowser(info.URL) },
		},
		mygo.Separator(),
	}
}
