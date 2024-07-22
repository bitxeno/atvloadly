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
	InstallAppQueue chan model.InstalledApp
	chExitQueue     chan bool
}

func new() *Task {
	return &Task{
		InstallAppQueue: make(chan model.InstalledApp, 1),
		chExitQueue:     make(chan bool, 1),
	}
}

func (t *Task) RunSchedule() error {
	if t.c != nil {
		t.Stop()
	}

	if !app.Settings.Task.Enabled {
		log.Info("App refresh scheduled task is not enabled")
		return nil
	}

	t.c = cron.New()
	if _, err := t.c.AddFunc(app.Settings.Task.CrodTime, t.Run); err != nil {
		log.Err(err).Msgf("Failed to start app refresh scheduled task due to incorrect timing format: %s", app.Settings.Task.CrodTime)
		t.c = nil
		return err
	}

	log.Infof("App refresh scheduled task has started, time: %s", app.Settings.Task.CrodTime)
	t.Start()

	return nil
}

func (t *Task) Start() {
	t.c.Start()
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

	log.Info("Start executing installation task...")
	for _, v := range installedApps {
		if !t.checkNeedRefresh(v) {
			continue
		}

		t.StartInstallApp(v)
	}
	log.Info("Installation task completed.")
}

func (t *Task) runQueue() {
	for {
		select {
		case v := <-t.InstallAppQueue:
			t.tryInstallApp(v)
			t.InstallingApps.Delete(v.ID)

			// Next execution delayed by 5 seconds.
			time.Sleep(5 * time.Second)
		case <-t.chExitQueue:
			log.Info("Install app queue exit.")
			return
		}
	}
}

func (t *Task) checkNeedRefresh(v model.InstalledApp) bool {
	now := time.Now()

	// fix RefreshedDate is nil
	if v.RefreshedDate == nil {
		return true
	}

	// refresh when the expiration time is less than one day.
	if app.Settings.Task.Mode == app.OneDayAgoMode {
		expireTime := v.RefreshedDate.AddDate(0, 0, 6)
		if expireTime.Before(now) {
			return true
		}
	}

	// today has refreshed will ignore
	if app.Settings.Task.Mode == app.CustomMode {
		if v.RefreshedDate.Format("2006-01-02") != now.Format("2006-01-02") {
			return true
		}
	}

	return false
}

func (t *Task) StartInstallApp(v model.InstalledApp) {
	go func() {
		t.InstallAppQueue <- v
	}()
}

func (t *Task) tryInstallApp(v model.InstalledApp) {
	log.Infof("Start installing ipa: %s", v.IpaName)
	err := t.runInternal(v)

	now := time.Now()
	if err == nil {
		v.RefreshedDate = &now
		v.RefreshedResult = true
		_ = service.UpdateAppRefreshResult(v)
		log.Infof("Installing ipa success: %s", v.IpaName)
	} else {
		v.RefreshedResult = false
		_ = service.UpdateAppRefreshResult(v)

		// Send installation failure notification
		title := i18n.LocalizeF("notify.title", map[string]interface{}{"name": v.IpaName})
		message := i18n.LocalizeF("notify.content", map[string]interface{}{"account": v.Account, "error": err.Error()})
		_ = notify.Send(title, message)
		log.Infof("Installing ipa failed: %s error: %s", v.IpaName, err.Error())
	}
}

func (t *Task) runInternal(v model.InstalledApp) error {
	if v.Account == "" || v.Password == "" || v.UDID == "" {
		log.Info("account or password or UDID is empty")
		return fmt.Errorf("account or password or UDID is empty")
	}

	installMgr := manager.NewInstallManager()
	defer installMgr.Close()
	err := installMgr.TryStart(context.Background(), v.UDID, v.Account, v.Password, v.IpaPath)
	installMgr.WriteLog(v.ID)
	if err != nil {
		log.Err(err).Msgf("Error executing installation script. %s", installMgr.ErrorLog())
		return err
	}
	if strings.Contains(installMgr.OutputLog(), "Installation Succeeded") {
		return nil
	} else {
		return fmt.Errorf(installMgr.ErrorLog())
	}
}

func ScheduleRefreshApps() error {
	return instance.RunSchedule()
}

func RunInstallApp(v model.InstalledApp) {
	if _, loaded := instance.InstallingApps.LoadOrStore(v.ID, v); !loaded {
		instance.StartInstallApp(v)
	}
}

func GetCurrentInstallingApps() []model.InstalledApp {
	installingApps := []model.InstalledApp{}

	instance.InstallingApps.Range(func(key, value interface{}) bool {
		installingApps = append(installingApps, value.(model.InstalledApp))
		return true
	})
	return installingApps
}

func ReloadTask() error {
	log.Info("Reload task...")
	return instance.RunSchedule()
}
