package task

import (
	"fmt"
	"strings"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/i18n"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/manager"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/notify"
	"github.com/bitxeno/atvloadly/internal/service"
)

// CheckUpdates checks the tracked apps for new builds and notifies what
// changed. It never installs: automatic updates are installed by Run, during
// the refresh time window.
func (t *Task) CheckUpdates() {
	installedApps, err := service.GetEnableAppList()
	if err != nil {
		log.Err(err).Msg("Failed to get the installation list")
		return
	}

	res := service.CheckSourceUpdates(installedApps)
	log.Infof("Update check finished: %d update(s) available.", len(res.Updates))
	sendSourceNotices(res.Notices)
}

// canInstallUpdate reports whether an update of v can be installed now.
func canInstallUpdate(v model.InstalledApp) bool {
	if v.IsAccountInvalid() {
		log.Warnf("The install account (%s) is invalid, skip update app: %s.", v.MaskAccount(), v.IpaName)
		return false
	}
	if _, found := manager.GetDeviceByUDID(v.UDID); !found {
		return false
	}
	// iPhone relies on whether the phone is unlocked
	if v.IsIPhoneApp() {
		if err := manager.CheckAfcServiceStatus(v.UDID); err != nil {
			return false
		}
	}
	return true
}

// sendSourceNotices sends the findings of an update check in one message.
func sendSourceNotices(notices []service.SourceNotice) {
	if len(notices) == 0 || !app.Settings.Notification.Enabled {
		return
	}

	var message strings.Builder
	for _, n := range notices {
		if n.Error == "" {
			message.WriteString(i18n.LocalizeF("notify.update_available", map[string]any{"name": n.AppName, "version": n.Version}))
		} else {
			message.WriteString(i18n.LocalizeF("notify.update_attention", map[string]any{"name": n.AppName, "error": n.Error}))
		}
	}
	_ = notify.Send(i18n.Localize("notify.update_title"), message.String())
}

// updateCheckSpec returns the cron spec of the update check every hours hours.
func updateCheckSpec(hours int) string {
	switch hours {
	case 1, 3, 6, 12:
		return fmt.Sprintf("17 */%d * * *", hours)
	case 24:
		return "17 0 * * *"
	default:
		return "17 */6 * * *"
	}
}

// UpdateApp queues the installation of target, an installed app with a new
// build applied (see service.PrepareSourceUpdate). It returns false when the
// app is already installing.
func UpdateApp(target model.InstalledApp) bool {
	return instance.StartInstallApps([]model.InstalledApp{target}, true) == 1
}
