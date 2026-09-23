package task

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/i18n"
	"github.com/bitxeno/atvloadly/internal/ipa"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/manager"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/notify"
	"github.com/bitxeno/atvloadly/internal/service"
	"github.com/robfig/cron/v3"
)

var instance = new()

type Task struct {
	c               *cron.Cron
	InstallingApps  sync.Map
	InstallAppQueue chan TaskItem
	chExitQueue     chan bool
	InvalidAccounts map[string]bool
	// RefreshingDevices prevents concurrent refresh operations for the same device UDID
	RefreshingDevices sync.Map
	// Batch tracking for aggregated notifications
	batchMu      sync.Mutex
	currentBatch *BatchInfo
}

type TaskItem struct {
	App     model.InstalledApp
	Notify  bool
	BatchID string
}

type BatchInfo struct {
	ID           string
	TotalCount   int
	SuccessCount int
	FailedApps   []FailedAppInfo
	UpdatedApps  []string // localized "updated to" lines of successful source updates
	Notify       bool
}

type FailedAppInfo struct {
	AppName string
	Account string
	Error   string
}

func new() *Task {
	return &Task{
		InstallAppQueue: make(chan TaskItem, 100),
		chExitQueue:     make(chan bool, 1),
	}
}

func (t *Task) RunSchedule() error {
	if t.c != nil {
		t.Stop()
	}

	if _, err := cron.ParseStandard(app.Settings.Task.CrodTime); err != nil {
		log.Err(err).Msgf("Failed to start app refresh scheduled task due to incorrect timing format: %s", app.Settings.Task.CrodTime)
		return err
	}

	t.c = cron.New()
	if app.Settings.Task.Enabled {
		if _, err := t.c.AddFunc(app.Settings.Task.CrodTime, t.Run); err != nil {
			t.c = nil
			return err
		}
	}
	if hours := app.Settings.Update.CheckInterval; hours > 0 {
		if _, err := t.c.AddFunc(updateCheckSpec(hours), t.CheckUpdates); err != nil {
			t.c = nil
			return err
		}
	}

	t.Start()

	return nil
}

func (t *Task) Start() {
	if app.Settings.Task.Enabled {
		log.Infof("App refresh scheduled task has started, time: %s", app.Settings.Task.CrodTime)
	} else {
		log.Warn("App refresh scheduled task is disabled.")
	}
	if hours := app.Settings.Update.CheckInterval; hours > 0 {
		log.Infof("App update check scheduled task has started, time: %s", updateCheckSpec(hours))
	}
	t.c.Start()

	// Register device connection callback to automatically refresh the application when the device is connected
	manager.SetDeviceConnectedCallback(func(device model.Device) {
		if err := t.autoRefreshDeviceApps(device); err != nil {
			log.Err(err).Msgf("Failed to refresh apps for device: %s (UDID: %s)", device.Name, device.UDID)
		}
	})

	go t.runQueue()
}

func (t *Task) Stop() {
	t.chExitQueue <- true
	<-t.c.Stop().Done()
	t.c = nil
}

func (t *Task) Run() {
	installedApps, err := service.GetEnableAppList()
	if err != nil {
		log.Err(err).Msg("Failed to get the installation list")
		return
	}

	res := service.CheckSourceUpdates(installedApps)
	sendSourceNotices(res.Notices)
	updates := make(map[uint]model.InstalledApp, len(res.Updates))
	for _, target := range res.Updates {
		updates[target.ID] = target
	}

	appsNeedRefresh := make([]model.InstalledApp, 0)
	updateCount := 0
	for _, v := range installedApps {
		// An update re-signs the app, so it replaces the refresh.
		if target, ok := updates[v.ID]; ok && v.Source.AutoUpdate && target.Source.BuildID != v.Source.FailedBuildID && canInstallUpdate(v) {
			appsNeedRefresh = append(appsNeedRefresh, target)
			updateCount++
			continue
		}

		if !v.NeedRefresh(app.Settings.Task.AdvanceDays) {
			continue
		}

		if v.IsAccountInvalid() {
			log.Warnf("The install account (%s) is invalid, skip refresh app: %s.", v.MaskAccount(), v.IpaName)
			continue
		}

		// iPhone cannot refresh on a schedule and relies on whether the phone is unlocked
		// Need to check Afc service status before refreshing
		if v.IsIPhoneApp() {
			if err := manager.CheckAfcServiceStatus(v.UDID); err != nil {
				continue
			}
		}

		appsNeedRefresh = append(appsNeedRefresh, v)
	}

	if len(appsNeedRefresh) == 0 {
		log.Info("No apps need to be refreshed.")
		return
	}

	log.Infof("Start executing installation task (%d need refresh, %d need update)...", len(appsNeedRefresh)-updateCount, updateCount)
	t.StartInstallApps(appsNeedRefresh, true)
}

// StartInstallApps queues apps for installation and returns how many were
// queued; apps already installing are skipped.
func (t *Task) StartInstallApps(apps []model.InstalledApp, notify bool) int {
	t.resetInvalidAccounts()

	if len(apps) == 0 {
		return 0
	}

	// Create a batch for aggregated notification
	batchID := fmt.Sprintf("batch-%d", time.Now().UnixNano())
	t.batchMu.Lock()
	t.currentBatch = &BatchInfo{
		ID:           batchID,
		TotalCount:   len(apps),
		SuccessCount: 0,
		FailedApps:   make([]FailedAppInfo, 0),
		Notify:       notify,
	}
	t.batchMu.Unlock()

	queued := 0
	for _, v := range apps {
		if t.startInstallAppInternal(v, notify, batchID) {
			queued++
		}
	}

	// The batch only waits for the queued apps.
	t.batchMu.Lock()
	if t.currentBatch != nil && t.currentBatch.ID == batchID {
		t.currentBatch.TotalCount = queued
		t.completeBatchIfDone()
	}
	t.batchMu.Unlock()

	return queued
}

func (t *Task) startInstallAppInternal(v model.InstalledApp, notify bool, batchID string) bool {
	if _, loaded := t.InstallingApps.LoadOrStore(v.ID, v); loaded {
		return false
	}
	select {
	case t.InstallAppQueue <- TaskItem{App: v, Notify: notify, BatchID: batchID}:
		return true
	default:
		t.InstallingApps.Delete(v.ID)
		log.Warnf("The install queue is full, skip task: %s", v.IpaName)
		return false
	}
}

func (t *Task) runQueue() {
	// Wait for one minute before install at startup to avoid the usbmuxd service not being ready.
	manager.Usbmuxd().TryWaitReady(30 * time.Second)

	for {
		select {
		case v := <-t.InstallAppQueue:
			t.tryInstallApp(v)
			t.InstallingApps.Delete(v.App.ID)

			// Next execution delayed by 10 seconds.
			time.Sleep(10 * time.Second)
		case <-t.chExitQueue:
			log.Info("Install app queue exit.")
			return
		}
	}
}

func (t *Task) tryInstallApp(item TaskItem) {
	// Decide before resolveIPA replaces a remote IpaPath with the downloaded file.
	refresh := shouldUseRefreshMode(item.App)
	update := isSourceUpdate(item.App)

	resolvedApp, err := t.resolveIPA(item.App)
	if err != nil {
		log.Err(err).Msgf("Prepare ipa path failed: %s", item.App.IpaName)
		t.handleInstallFailure(item, item.App, err)
		return
	}
	v := *resolvedApp

	log.Infof("Start installing ipa: %s", v.IpaName)
	installMgr := manager.NewInstallManager()
	defer func() {
		installMgr.SaveLog(v.ID)
		installMgr.CleanTempFiles(v.IpaPath)
		installMgr.Close()
	}()
	provisioningProfile, err := t.runInternal(v, refresh, installMgr)

	success := err == nil
	if success {
		now := time.Now()
		expirationDate := now.AddDate(0, 0, 7)
		if provisioningProfile != nil {
			expirationDate = provisioningProfile.ExpirationDate.Local()
		}

		v.RefreshedDate = &now
		v.ExpirationDate = &expirationDate
		v.RefreshedResult = true
		v.RefreshedError = model.RefreshedErrorNone

		if v.ID == 0 || update {
			savedApp, saveErr := service.SaveApp(v)
			if saveErr != nil {
				log.Err(saveErr).Msgf("Save app failed after installation success: %s", v.IpaName)
				t.handleInstallFailure(item, v, saveErr)
				return
			}
			v = *savedApp
		} else {
			if updateErr := service.UpdateAppRefreshResult(v); updateErr != nil {
				log.Err(updateErr).Msgf("Update app refresh result failed: %s", v.IpaName)
			}
		}

		log.Infof("Installing ipa success: %s", v.IpaName)
	} else {
		t.handleInstallFailure(item, v, err)
		return
	}

	// Track batch progress and send aggregated notification
	t.trackBatchProgress(item, success, err)
}

func (t *Task) handleInstallFailure(item TaskItem, v model.InstalledApp, err error) {
	log.Err(err).Msgf("Installing ipa failed: %s", v.IpaName)
	v.RefreshedResult = false
	if errors.Is(err, manager.ErrAccountInvalid) {
		v.RefreshedError = model.RefreshedErrorInvalidAccount
	} else {
		v.RefreshedError = model.RefreshedErrorInvalidOther
	}
	if isSourceUpdate(item.App) {
		// The installed app is unchanged: only remember the failed build, and
		// an invalid account so that refreshes skip it.
		if markErr := service.MarkSourceUpdateFailed(item.App.ID, item.App.Source.BuildID, err.Error()); markErr != nil {
			log.Err(markErr).Msgf("Save update failure failed: %s", v.IpaName)
		}
		if v.RefreshedError == model.RefreshedErrorInvalidAccount {
			_ = service.UpdateAppRefreshResult(v)
		}
	} else if v.ID != 0 {
		_ = service.UpdateAppRefreshResult(v)
	}

	// Track batch progress and send aggregated notification
	t.trackBatchProgress(item, false, err)
}

func (t *Task) resolveIPA(v model.InstalledApp) (*model.InstalledApp, error) {
	remote := ipa.IsRemoteURL(v.IpaPath)
	// A refresh re-uses the stored IPA, return directly to avoid unnecessary download
	if v.ID != 0 && !remote {
		return &v, nil
	}

	var result *ipa.DownloadResult
	var err error
	if remote {
		result, err = ipa.DownloadAndParse(v.IpaPath, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to download ipa: %w", err)
		}
	} else {
		result, err = ipa.ParseLocalIPA(v.IpaPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ipa file: %w", err)
		}
	}

	if err := verifyIPA(v, result); err != nil {
		if remote {
			_ = os.Remove(result.LocalPath)
		}
		if result.IconPath != "" {
			_ = os.Remove(result.IconPath)
		}
		return nil, err
	}

	v.IpaPath = result.LocalPath
	v.IpaName = result.Name
	v.BundleIdentifier = result.BundleIdentifier
	v.Version = result.Version
	v.Icon = result.IconPath
	return &v, nil
}

// verifyIPA checks that the IPA can be installed as v: it must run on the
// device, and an update must keep the bundle identifier of the installed app.
func verifyIPA(v model.InstalledApp, result *ipa.DownloadResult) error {
	if err := ipa.CheckPlatform(result.Platforms, v.DeviceClass); err != nil {
		return err
	}
	if v.ID != 0 && result.BundleIdentifier != v.BundleIdentifier {
		return fmt.Errorf("bundle identifier changed from %s to %s; install it as a new app", v.BundleIdentifier, result.BundleIdentifier)
	}
	return nil
}

func (t *Task) trackBatchProgress(item TaskItem, success bool, err error) {
	t.batchMu.Lock()
	defer t.batchMu.Unlock()

	if t.currentBatch == nil || t.currentBatch.ID != item.BatchID {
		return
	}

	if success {
		t.currentBatch.SuccessCount++
		if isSourceUpdate(item.App) {
			t.currentBatch.UpdatedApps = append(t.currentBatch.UpdatedApps, i18n.LocalizeF("notify.update_installed", map[string]any{
				"name":    item.App.DisplayName(),
				"version": item.App.Source.Version,
			}))
		}
	} else {
		t.currentBatch.FailedApps = append(t.currentBatch.FailedApps, FailedAppInfo{
			AppName: item.App.IpaName,
			Account: item.App.Account,
			Error:   err.Error(),
		})
	}

	t.completeBatchIfDone()
}

// completeBatchIfDone sends the notification of the current batch and clears
// it once every queued app is done. batchMu must be held.
func (t *Task) completeBatchIfDone() {
	completedCount := t.currentBatch.SuccessCount + len(t.currentBatch.FailedApps)
	if completedCount >= t.currentBatch.TotalCount {
		// Batch complete, send aggregated notification
		t.sendBatchNotification(t.currentBatch)
		t.currentBatch = nil
	}
}

func (t *Task) sendBatchNotification(batch *BatchInfo) {
	if !batch.Notify || !app.Settings.Notification.Enabled {
		return
	}

	if len(batch.FailedApps) > 0 {
		// Some apps failed, send aggregated failure notification
		var message strings.Builder
		for _, failed := range batch.FailedApps {
			message.WriteString(i18n.LocalizeF("notify.batch_content", map[string]any{"name": failed.AppName, "error": failed.Error}))
		}
		title := i18n.LocalizeF("notify.batch_title", map[string]any{})
		_ = notify.Send(title, message.String())
	}

	if len(batch.UpdatedApps) > 0 {
		_ = notify.Send(i18n.Localize("notify.update_installed_title"), strings.Join(batch.UpdatedApps, ""))
	}
}

func (t *Task) runInternal(v model.InstalledApp, refresh bool, installMgr *manager.InstallManager) (*model.MobileProvisioningProfile, error) {
	if v.Account == "" || v.UDID == "" {
		installMgr.WriteLog("account or UDID is empty")
		return nil, fmt.Errorf("%s", "account or UDID is empty")
	}

	if _, ok := t.InvalidAccounts[v.Account]; ok {
		log.Warnf("The install account (%s) is invalid, skip install app: %s.", v.MaskAccount(), v.IpaName)
		installMgr.WriteLog(fmt.Sprintf("The install account (%s) is invalid, skip install.", v.MaskAccount()))
		return nil, fmt.Errorf("the install account (%s) is invalid, skip install", v.MaskAccount())
	}

	dev, found := manager.GetDeviceByUDID(v.UDID)
	if !found || dev == nil {
		return nil, fmt.Errorf("device not found for UDID: %s", v.UDID)
	}

	err := installMgr.TryStart(context.Background(), manager.InstallOptions{
		UDID:             v.UDID,
		Account:          v.Account,
		IP:               dev.IP,
		Port:             dev.Port,
		IpaPath:          v.IpaPath,
		CustomName:       v.CustomName,
		RemoveExtensions: v.RemoveExtensions,
		RefreshMode:      refresh,
	})
	if err != nil {
		installMgr.WriteLog(err.Error())
		if errors.Is(err, manager.ErrAccountInvalid) {
			t.InvalidAccounts[v.Account] = true
			return nil, err
		}
		return nil, fmt.Errorf("%s %s", installMgr.ErrorLog(), err.Error())
	}

	if installMgr.IsSuccess() {
		return installMgr.ProvisioningProfile, nil
	} else {
		return nil, fmt.Errorf("install failed with unknown error. %s", installMgr.ErrorLog())
	}
}

// shouldUseRefreshMode reports whether v is a refresh of an installed app,
// which re-uses its stored IPA and only renews the provisioning profiles.
func shouldUseRefreshMode(v model.InstalledApp) bool {
	return v.ID != 0 && !ipa.IsRemoteURL(v.IpaPath)
}

// isSourceUpdate reports whether v installs a new build of an installed app
// (see service.ApplyBuild), which must be signed again.
func isSourceUpdate(v model.InstalledApp) bool {
	return v.ID != 0 && ipa.IsRemoteURL(v.IpaPath)
}

func (t *Task) autoRefreshDeviceApps(device model.Device) error {
	if !device.IsIPhone() {
		return nil
	}
	if !app.Settings.Task.Enabled || !app.Settings.Task.IphoneEnabled {
		return nil
	}
	return t.refreshDeviceApps(device)
}

// refreshes apps when device is discovery on network, for iPhone only.
func (t *Task) refreshDeviceApps(device model.Device) error {
	deviceApps, err := service.GetEnableAppListByUDID(device.UDID)
	if err != nil {
		return err
	}

	appsNeedRefresh := make([]model.InstalledApp, 0)
	for _, v := range deviceApps {
		if !v.NeedRefresh(app.Settings.Task.AdvanceDays) {
			continue
		}

		if v.IsAccountInvalid() {
			log.Warnf("The install account (%s) is invalid, skip refresh app: %s.", v.MaskAccount(), v.IpaName)
			continue
		}

		appsNeedRefresh = append(appsNeedRefresh, v)
	}

	if len(appsNeedRefresh) == 0 {
		return nil
	}

	// Prevent concurrent refresh for the same device UDID
	if _, loaded := t.RefreshingDevices.LoadOrStore(device.UDID, true); loaded {
		return nil
	}

	go func(udid, name string, apps []model.InstalledApp) {
		// Ensure we clear the refreshing flag when finished
		defer t.RefreshingDevices.Delete(udid)

		// The iPhone may connect and disconnect instantly (for example, briefly lighting up the screen when receiving a message).
		// Check to ensure the device can connect truly.
		time.Sleep(30 * time.Second)
		if _, found := manager.GetDeviceByUDID(udid); !found {
			return
		}
		if err := manager.CheckAfcServiceStatus(udid); err != nil {
			log.Err(err).Msgf("Check AFC service status failed, skip refresh device: %s.", udid)
			return
		}

		log.Infof("Start refresh apps for device: %s (found %d apps, %d need refresh)...", name, len(deviceApps), len(apps))
		t.StartInstallApps(apps, false)
	}(device.UDID, device.Name, appsNeedRefresh)
	return nil
}

func (t *Task) resetInvalidAccounts() {
	t.InvalidAccounts = make(map[string]bool)
}

func ScheduleRefreshApps() error {
	return instance.RunSchedule()
}

func RefreshApp(v model.InstalledApp) {
	instance.StartInstallApps([]model.InstalledApp{v}, true)
}

// StartInstallApps queues apps for installation and returns how many were queued.
func StartInstallApps(apps []model.InstalledApp, notify bool) int {
	return instance.StartInstallApps(apps, notify)
}

func GetCurrentInstallingApps() []model.InstalledApp {
	installingApps := []model.InstalledApp{}

	instance.InstallingApps.Range(func(key, value any) bool {
		installingApps = append(installingApps, value.(model.InstalledApp))
		return true
	})
	return installingApps
}

func ReloadTask() error {
	log.Info("Reload task...")
	return instance.RunSchedule()
}

func RefreshDeviceApps(device model.Device) error {
	return instance.refreshDeviceApps(device)
}
