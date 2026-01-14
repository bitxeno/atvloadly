package task

import (
	"context"
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
}

type TaskItem struct {
	App    model.InstalledApp
	Notify bool
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
		if err := t.refreshDeviceApps(device); err != nil {
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
	t.InvalidAccounts = make(map[string]bool)
	installedApps, err := service.GetEnableAppList()
	if err != nil {
		log.Err(err).Msg("Failed to get the installation list")
		return
	}

	appsNeedRefresh := make([]model.InstalledApp, 0)
	for _, v := range installedApps {
		if !v.NeedRefresh() {
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
	for _, v := range appsNeedRefresh {
		t.StartInstallApp(v)
	}
	log.Info("Installation task completed.")
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

func (t *Task) StartInstallApp(v model.InstalledApp) {
	t.startInstallAppInternal(v, true)
}

func (t *Task) startInstallAppInternal(v model.InstalledApp, notify bool) {
	if _, loaded := t.InstallingApps.LoadOrStore(v.ID, v); !loaded {
		select {
		case t.InstallAppQueue <- TaskItem{App: v, Notify: notify}:
		default:
			t.InstallingApps.Delete(v.ID)
			log.Warnf("The install queue is full, skip task: %s", v.IpaName)
		}
	}
}

func (t *Task) tryInstallApp(item TaskItem) {
	v := item.App
	log.Infof("Start installing ipa: %s", v.IpaName)
	provisioningProfile, err := t.runInternal(v)

	now := time.Now()
	expirationDate := now.AddDate(0, 0, 7)
	if provisioningProfile != nil {
		expirationDate = provisioningProfile.ExpirationDate.Local()
	}
	if err == nil {
		v.RefreshedDate = &now
		v.ExpirationDate = &expirationDate
		v.RefreshedResult = true
		_ = service.UpdateAppRefreshResult(v)
		log.Infof("Installing ipa success: %s", v.IpaName)
	} else {
		v.RefreshedResult = false
		_ = service.UpdateAppRefreshResult(v)

		// Send installation failure notification
		if item.Notify && app.Settings.Notification.Enabled {
			title := i18n.LocalizeF("notify.title", map[string]any{"name": v.IpaName})
			message := i18n.LocalizeF("notify.content", map[string]any{"account": v.Account, "error": err.Error()})
			_ = notify.Send(title, message)
			log.Infof("Installing ipa failed: %s error: %s", v.IpaName, err.Error())
		}
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
		installMgr.WriteLog(fmt.Sprintf("The install account (%s) is invalid, skip install.", v.MaskAccount()))
		return nil, fmt.Errorf("The install account (%s) is invalid, skip install.", v.MaskAccount())
	}

	err := installMgr.TryStart(context.Background(), v.UDID, v.Account, v.Password, v.IpaPath, v.RemoveExtensions)
	if err != nil {
		log.Err(err).Msgf("Error executing installation script. %s", installMgr.ErrorLog())
		installMgr.WriteLog(err.Error())
		if strings.Contains(installMgr.OutputLog(), "Can't log-in") || strings.Contains(installMgr.OutputLog(), "DeveloperSession creation failed") {
			t.InvalidAccounts[v.Account] = true
		}
		return nil, fmt.Errorf("%s %s", installMgr.ErrorLog(), err.Error())
	}

	if strings.Contains(installMgr.OutputLog(), "Installation Succeeded") || strings.Contains(installMgr.OutputLog(), "Installation complete") {
		return installMgr.ProvisioningProfile, nil
	} else {
		if strings.Contains(installMgr.OutputLog(), "Can't log-in") || strings.Contains(installMgr.OutputLog(), "DeveloperSession creation failed") {
			t.InvalidAccounts[v.Account] = true
		}
		return nil, fmt.Errorf("%s", installMgr.ErrorLog())
	}
}

// refreshes apps when device is discovery on network, for iPhone only.
func (t *Task) refreshDeviceApps(device model.Device) error {
	if !device.IsIPhone() {
		return nil
	}
	if !app.Settings.Task.Enabled || !app.Settings.Task.IphoneEnabled {
		return nil
	}

	deviceApps, err := service.GetEnableAppListByUDID(device.UDID)
	if err != nil {
		return err
	}

	appsNeedRefresh := make([]model.InstalledApp, 0)
	for _, v := range deviceApps {
		if v.NeedRefresh() {
			appsNeedRefresh = append(appsNeedRefresh, v)
		}
	}

	if len(appsNeedRefresh) == 0 {
		return nil
	}

	// Prevent concurrent refresh for the same device UDID
	if _, loaded := t.RefreshingDevices.LoadOrStore(device.UDID, true); loaded {
		log.Infof("Device refresh already in progress, skip: %s (UDID: %s)", device.Name, device.UDID)
		return nil
	}

	go func(udid, name string, apps []model.InstalledApp) {
		// Ensure we clear the refreshing flag when finished
		defer t.RefreshingDevices.Delete(udid)

		// The iPhone may connect and disconnect instantly (for example, briefly lighting up the screen when receiving a message).
		// Check to ensure the device can connect truly.
		time.Sleep(30 * time.Second)
		if err := manager.CheckAfcServiceStatus(udid); err != nil {
			return
		}

		log.Infof("Start refresh apps for device: %s (found %d apps, %d need refresh)...", name, len(deviceApps), len(apps))
		for _, v := range apps {
			t.startInstallAppInternal(v, false)
		}
	}(device.UDID, device.Name, appsNeedRefresh)
	return nil
}

func ScheduleRefreshApps() error {
	return instance.RunSchedule()
}

func RunInstallApp(v model.InstalledApp) {
	instance.StartInstallApp(v)
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
