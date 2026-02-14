package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/i18n"
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

	t.c = cron.New()
	if _, err := t.c.AddFunc(app.Settings.Task.CrodTime, t.Run); err != nil {
		log.Err(err).Msgf("Failed to start app refresh scheduled task due to incorrect timing format: %s", app.Settings.Task.CrodTime)
		t.c = nil
		return err
	}

	t.Start()

	return nil
}

func (t *Task) Start() {
	if app.Settings.Task.Enabled {
		log.Infof("App refresh scheduled task has started, time: %s", app.Settings.Task.CrodTime)
		t.c.Start()
	} else {
		log.Warn("App refresh scheduled task is disabled.")
	}

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

	appsNeedRefresh := make([]model.InstalledApp, 0)
	for _, v := range installedApps {
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

	log.Infof("Start executing installation task (%d need refresh)...", len(appsNeedRefresh))
	t.StartInstallApps(appsNeedRefresh, true)
}

func (t *Task) StartInstallApps(apps []model.InstalledApp, notify bool) {
	t.resetInvalidAccounts()

	if len(apps) == 0 {
		return
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

	for _, v := range apps {
		t.startInstallAppInternal(v, notify, batchID)
	}
}

func (t *Task) startInstallAppInternal(v model.InstalledApp, notify bool, batchID string) {
	if _, loaded := t.InstallingApps.LoadOrStore(v.ID, v); !loaded {
		select {
		case t.InstallAppQueue <- TaskItem{App: v, Notify: notify, BatchID: batchID}:
		default:
			t.InstallingApps.Delete(v.ID)
			log.Warnf("The install queue is full, skip task: %s", v.IpaName)
		}
	}
}

func (t *Task) runQueue() {
	// Wait for one minute before install at startup to avoid the usbmuxd service not being ready.
	time.Sleep(time.Minute)

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
	v := item.App
	log.Infof("Start installing ipa: %s", v.IpaName)
	provisioningProfile, err := t.runInternal(v)

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
		_ = service.UpdateAppRefreshResult(v)
		log.Infof("Installing ipa success: %s", v.IpaName)
	} else {
		log.Err(err).Msgf("Installing ipa failed: %s", v.IpaName)
		v.RefreshedResult = false
		if errors.Is(err, manager.ErrAccountInvalid) {
			v.RefreshedError = model.RefreshedErrorInvalidAccount
		} else {
			v.RefreshedError = model.RefreshedErrorInvalidOther
		}
		_ = service.UpdateAppRefreshResult(v)
	}

	// Track batch progress and send aggregated notification
	t.trackBatchProgress(item, success, err)
}

func (t *Task) trackBatchProgress(item TaskItem, success bool, err error) {
	t.batchMu.Lock()
	defer t.batchMu.Unlock()

	if t.currentBatch == nil || t.currentBatch.ID != item.BatchID {
		return
	}

	if success {
		t.currentBatch.SuccessCount++
	} else {
		t.currentBatch.FailedApps = append(t.currentBatch.FailedApps, FailedAppInfo{
			AppName: item.App.IpaName,
			Account: item.App.Account,
			Error:   err.Error(),
		})
	}

	// Check if batch is complete
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
}

func (t *Task) runInternal(v model.InstalledApp) (*model.MobileProvisioningProfile, error) {
	installMgr := manager.NewInstallManager()
	defer func() {
		installMgr.SaveLog(v.ID)
		installMgr.Close()
	}()

	if v.Account == "" || v.UDID == "" {
		installMgr.WriteLog("account or UDID is empty")
		return nil, fmt.Errorf("%s", "account or UDID is empty")
	}

	if _, ok := t.InvalidAccounts[v.Account]; ok {
		log.Warnf("The install account (%s) is invalid, skip install app: %s.", v.MaskAccount(), v.IpaName)
		installMgr.WriteLog(fmt.Sprintf("The install account (%s) is invalid, skip install.", v.MaskAccount()))
		return nil, fmt.Errorf("The install account (%s) is invalid, skip install.", v.MaskAccount())
	}

	err := installMgr.TryStart(context.Background(), manager.InstallOptions{
		UDID:             v.UDID,
		Account:          v.Account,
		Password:         v.Password,
		IpaPath:          v.IpaPath,
		RemoveExtensions: v.RemoveExtensions,
		RefreshMode:      true,
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
