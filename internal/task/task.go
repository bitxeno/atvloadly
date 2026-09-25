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
	"github.com/bitxeno/atvloadly/internal/signing"
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
	// ExternalSkipLogged records the external certificate apps whose skipped
	// automatic refresh was already logged, to log it once per app.
	ExternalSkipLogged sync.Map
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

	// Recover keeps a panicking job (a refresh or an update check) from
	// taking the whole service down.
	t.c = cron.New(cron.WithChain(cron.Recover(cron.DefaultLogger)))
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
	// An update re-signs the app, so it replaces the refresh.
	notUpdated := make([]model.InstalledApp, 0, len(installedApps))
	for _, v := range installedApps {
		if target, ok := updates[v.ID]; ok && v.Source.AutoUpdate && target.Source.BuildID != v.Source.FailedBuildID && canInstallUpdate(v) {
			appsNeedRefresh = append(appsNeedRefresh, target)
			continue
		}
		notUpdated = append(notUpdated, v)
	}
	updateCount := len(appsNeedRefresh)
	for _, v := range t.autoRefreshApps(notUpdated) {
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

	// Create a batch for aggregated notification. batchMu is held while
	// queueing so that no queued app finishes before its batch is current.
	batchID := fmt.Sprintf("batch-%d", time.Now().UnixNano())
	t.batchMu.Lock()
	defer t.batchMu.Unlock()

	queued := 0
	for _, v := range apps {
		if t.startInstallAppInternal(v, notify, batchID) {
			queued++
		}
	}

	// The batch only waits for the queued apps. When none was queued (they
	// are already installing), the batch in flight stays current so that its
	// notification is still sent.
	if queued > 0 {
		t.currentBatch = &BatchInfo{
			ID:           batchID,
			TotalCount:   queued,
			SuccessCount: 0,
			FailedApps:   make([]FailedAppInfo, 0),
			Notify:       notify,
		}
	}

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
	// The downloaded or uploaded files of a new installation or an update,
	// before SaveApp moves them. A reinstallation uses the files of its record,
	// which must stay.
	var uploadedIPA, uploadedIcon string
	if v.ID == 0 || update {
		uploadedIPA, uploadedIcon = v.IpaPath, v.Icon
	}

	log.Infof("Start installing ipa: %s", v.IpaName)
	installMgr := manager.NewInstallManager()
	defer func() {
		installMgr.SaveLog(v.ID)
		if v.IsExternalSigning() {
			// Never the global temporary files: other installations may use them.
			service.CleanExternalUpload(uploadedIPA, uploadedIcon)
		} else {
			installMgr.CleanTempFiles(v.IpaPath)
		}
		installMgr.Close()
	}()
	result, err := t.runInternal(v, refresh, installMgr)

	success := err == nil
	if success {
		now := time.Now()
		expirationDate := result.ExpirationDate

		v.RefreshedDate = &now
		v.ExpirationDate = &expirationDate
		v.RefreshedResult = true
		v.RefreshedError = model.RefreshedErrorNone
		v.SignedBundleIdentifier = result.SignedBundleIdentifier

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
	v.RefreshedError = service.RefreshedErrorOf(err)
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
			Error:   failureDescription(err),
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

// installResult is what a successful installation records on the app.
type installResult struct {
	ExpirationDate time.Time
	// SignedBundleIdentifier is set by external certificate installations.
	SignedBundleIdentifier string
}

func (t *Task) runInternal(v model.InstalledApp, refresh bool, installMgr *manager.InstallManager) (*installResult, error) {
	if v.IsExternalSigning() {
		return t.runExternal(v, installMgr)
	}
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
		expirationDate := time.Now().AddDate(0, 0, 7)
		if installMgr.ProvisioningProfile != nil {
			expirationDate = installMgr.ProvisioningProfile.ExpirationDate.Local()
		}
		return &installResult{ExpirationDate: expirationDate}, nil
	} else {
		return nil, fmt.Errorf("install failed with unknown error. %s", installMgr.ErrorLog())
	}
}

// runExternal installs v with its external signing identity. A refresh of an
// installed app is a full reinstallation with the same identity: the engine
// refresh mode belongs to Apple ID sessions, and the app keeps the deadline of
// the identity. The invalid account bookkeeping does not apply.
func (t *Task) runExternal(v model.InstalledApp, installMgr *manager.InstallManager) (*installResult, error) {
	if v.UDID == "" {
		installMgr.WriteLog("UDID is empty")
		return nil, fmt.Errorf("%s", "UDID is empty")
	}

	dev, found := manager.GetDeviceByUDID(v.UDID)
	if !found || dev == nil {
		return nil, signing.Errorf(signing.ClassTransport, signing.CodeDeviceUnreachable, "device not found for UDID: %s", v.UDID)
	}

	result, err := service.RunExternalInstall(context.Background(), installMgr, v, dev, true)
	if err != nil {
		return nil, err
	}
	return &installResult{
		ExpirationDate:         result.ExpiresAt.Local(),
		SignedBundleIdentifier: result.SignedBundleIdentifier,
	}, nil
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

// failureDescription is the notification text of an installation failure.
// External certificate failures name what the user has to fix.
func failureDescription(err error) string {
	switch signing.ClassOf(err) {
	case signing.ClassIdentity:
		return "signing identity error: " + err.Error()
	case signing.ClassSigning:
		return "signing error: " + err.Error()
	case signing.ClassTransport:
		return "device error: " + err.Error()
	default:
		return err.Error()
	}
}

// selectAutoRefreshApps splits the apps an automatic refresh looks at: refresh
// holds the Apple ID apps due for a refresh whose account is valid; external
// holds the external certificate apps due for a refresh, which cannot be
// refreshed automatically (the identity deadline does not move).
func selectAutoRefreshApps(apps []model.InstalledApp, advanceDays int) (refresh, external []model.InstalledApp) {
	refresh = make([]model.InstalledApp, 0)
	for _, v := range apps {
		if !v.NeedRefresh(advanceDays) {
			continue
		}
		if v.IsExternalSigning() {
			external = append(external, v)
			continue
		}
		if v.IsAccountInvalid() {
			log.Warnf("The install account (%s) is invalid, skip refresh app: %s.", v.MaskAccount(), v.IpaName)
			continue
		}
		refresh = append(refresh, v)
	}
	return refresh, external
}

// autoRefreshApps returns the apps an automatic refresh renews and logs, once
// per app, the external certificate apps it skips.
func (t *Task) autoRefreshApps(apps []model.InstalledApp) []model.InstalledApp {
	refresh, external := selectAutoRefreshApps(apps, app.Settings.Task.AdvanceDays)
	for _, v := range external {
		if _, logged := t.ExternalSkipLogged.LoadOrStore(v.ID, true); !logged {
			log.Warnf("App %s is signed with an external certificate and is not refreshed automatically: reinstalling it does not extend its validity. Replace the provisioning profile or import a new signing identity, then reinstall it.", v.IpaName)
		}
	}
	return refresh
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

	appsNeedRefresh := t.autoRefreshApps(deviceApps)

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

// IsInstalling reports whether app id is queued or installing.
func IsInstalling(id uint) bool {
	return instance.isInstalling(id)
}

func (t *Task) isInstalling(id uint) bool {
	_, ok := t.InstallingApps.Load(id)
	return ok
}

func ReloadTask() error {
	log.Info("Reload task...")
	return instance.RunSchedule()
}

func RefreshDeviceApps(device model.Device) error {
	return instance.refreshDeviceApps(device)
}
