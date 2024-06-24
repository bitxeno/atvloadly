package task

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	c             *cron.Cron
	Running       bool `json:"running"`
	InstallingApp *model.InstalledApp
}

func new() *Task {
	return &Task{}
}

func (t *Task) RunSchedule() error {
	if t.c != nil {
		t.c.Stop()
		t.c = nil
	}

	if !app.Settings.Task.Enabled {
		log.Info(i18n.Localize("task.task_disabled"))
		return nil
	}

	t.c = cron.New()
	if _, err := t.c.AddFunc(app.Settings.Task.CrodTime, t.Run); err != nil {
		log.Err(err).Msg(i18n.LocalizeF("task.error_invalid_crod_time_format", map[string]interface{}{"time": app.Settings.Task.CrodTime}))
		t.c = nil
		return err
	}

	log.Info(i18n.LocalizeF("task.task_started", map[string]interface{}{"time": app.Settings.Task.CrodTime}))
	t.c.Start()

	return nil
}

func (t *Task) Stop() {
	t.c.Stop()
}

func (t *Task) Run() {
	if t.Running {
		return
	}
	t.startedState()
	defer t.completedState()

	installedApps, err := service.GetEnableAppList()
	if err != nil {
		log.Err(err).Msg(i18n.Localize("task.error_get_app_list"))
		return
	}

	now := time.Now()
	failedList := []model.InstalledApp{}
	failedMsg := ""
	for _, v := range installedApps {
		if !t.checkNeedRefresh(v) {
			continue
		}

		log.Info(i18n.LocalizeF("task.app_install_started", map[string]interface{}{"name": v.IpaName}))
		err := t.runInternalRetry(v)
		if err != nil {
			now := time.Now()
			v.RefreshedDate = &now
			v.RefreshedResult = false
			_ = service.UpdateAppRefreshResult(v)

			failedList = append(failedList, v)
			failedMsg += i18n.LocalizeF("notify.batch_content", map[string]interface{}{"name": v.IpaName, "error": err.Error()})
		} else {
			v.RefreshedDate = &now
			v.RefreshedResult = true
			_ = service.UpdateAppRefreshResult(v)
		}
		log.Info(i18n.LocalizeF("task.app_install_completed", map[string]interface{}{"name": v.IpaName}))

		// Next execution delayed by 10 seconds.
		time.Sleep(10 * time.Second)
	}

	// Send installation failure notification.
	if len(failedList) > 0 {
		title := i18n.LocalizeF("notify.title", map[string]interface{}{"name": "atvloadly"})
		_ = notify.Send(title, failedMsg)
	}
}

func (t *Task) checkNeedRefresh(v model.InstalledApp) bool {
	now := time.Now()

	// 过期时间少于一天时，再安装
	if app.Settings.Task.Mode == app.OneDayAgoMode {
		expireTime := v.RefreshedDate.AddDate(0, 0, 6)
		if expireTime.Before(now) {
			return true
		}
	}

	// 每天安装
	if app.Settings.Task.Mode == app.DailyMode {
		if v.RefreshedDate.Format("2006-01-02") != now.Format("2006-01-02") {
			return true
		}
	}

	return false
}

func (t *Task) RunImmediately(v model.InstalledApp) {
	if t.Running {
		return
	}
	t.startedState()
	defer t.completedState()

	now := time.Now()
	err := t.runInternalRetry(v)
	if err == nil {
		v.RefreshedDate = &now
		v.RefreshedResult = true
		_ = service.UpdateAppRefreshResult(v)
	} else {
		v.RefreshedDate = &now
		v.RefreshedResult = false
		_ = service.UpdateAppRefreshResult(v)

		// Send installation failure notification
		title := i18n.LocalizeF("notify.title", map[string]interface{}{"name": v.IpaName})
		message := i18n.LocalizeF("notify.content", map[string]interface{}{"account": v.Account, "error": err.Error()})
		_ = notify.Send(title, message)
	}
}

func (t *Task) runInternalRetry(v model.InstalledApp) error {
	err := t.runInternal(v)
	// AppleTV system has reboot/lockdownd sleep, try restart usbmuxd to fix
	// LOCKDOWN_E_MUX_ERROR / AFC_E_MUX_ERROR /
	if err != nil {
		log.Info(i18n.LocalizeF("task.try_restart_usbmuxd", map[string]interface{}{"name": v.IpaName}))
		if err = manager.RestartUsbmuxd(); err == nil {
			log.Info(i18n.LocalizeF("task.try_restart_usbmuxd_success", map[string]interface{}{"name": v.IpaName}))
			time.Sleep(5 * time.Second)
			err = t.runInternal(v)
		}
	}
	return err
}

func (t *Task) runInternal(v model.InstalledApp) error {
	t.InstallingApp = &v

	if v.Account == "" || v.Password == "" || v.UDID == "" {
		log.Info(i18n.Localize("task.error_invalid_arguments"))
		return fmt.Errorf(i18n.Localize("task.error_invalid_arguments"))
	}

	// The sideloader will handle special character "$". For those with this special character, it needs to be enclosed in single quotation marks.
	cmd := exec.Command("sideloader", "install", "--quiet", "--nocolor", "--udid", v.UDID, "-a", v.Account, "-p", v.Password, v.IpaPath)
	cmd.Dir = app.Config.Server.DataDir
	cmd.Env = []string{"SIDELOADER_CONFIG_DIR=" + app.SideloaderDataDir()}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Err(err).Msg("Error obtaining stdin: ")
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Err(err).Msg("Error obtaining stdout: ")
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Err(err).Msg("Error obtaining stdout: ")
		return err
	}

	var output strings.Builder
	var outputErr strings.Builder
	reader := bufio.NewReader(stdout)
	readerErr := bufio.NewReader(stderr)
	go func(reader io.Reader) {
		defer stdin.Close()
		scanner := bufio.NewScanner(reader)

		for scanner.Scan() {
			lineText := scanner.Text()
			_, _ = output.WriteString(lineText)
			_, _ = output.WriteString("\n")

			// Processing interaction to continue, such as [the Installing AltStore with Multiple AltServers the Not Supported] message.
			if strings.Contains(lineText, "Press any key to continue") {
				_, _ = stdin.Write([]byte("\n"))
			}
		}
	}(reader)
	go func(reader io.Reader) {
		defer stdin.Close()
		scanner := bufio.NewScanner(reader)

		for scanner.Scan() {
			lineText := scanner.Text()
			_, _ = output.WriteString(lineText)
			_, _ = output.WriteString("\n")

			_, _ = outputErr.WriteString(lineText)
			_, _ = outputErr.WriteString("\n")

			// Processing interaction to continue, such as [the Installing AltStore with Multiple AltServers the Not Supported] message.
			if strings.Contains(lineText, "Press any key to continue") {
				_, _ = stdin.Write([]byte("\n"))
			}
		}
	}(readerErr)
	if err := cmd.Start(); nil != err {
		data := []byte(output.String())
		t.writeLog(v, data)
		log.Err(err).Msg(i18n.LocalizeF("install_failed", map[string]interface{}{"error": outputErr.String()}))
		return fmt.Errorf("%s %v", outputErr.String(), err)
	}

	err = cmd.Wait()
	if err != nil {
		data := []byte(output.String())
		t.writeLog(v, data)
		log.Err(err).Msg(i18n.LocalizeF("install_failed", map[string]interface{}{"error": outputErr.String()}))
		return fmt.Errorf("%s %v", outputErr.String(), err)
	}

	data := []byte(output.String())
	t.writeLog(v, data)
	if strings.Contains(string(data), "Installation Succeeded") {
		return nil
	} else {
		return fmt.Errorf(outputErr.String())
	}
}

func (t *Task) writeLog(v model.InstalledApp, data []byte) {
	// Hide log password string
	data = bytes.Replace(data, []byte(v.Password), []byte("******"), -1)

	saveDir := filepath.Join(app.Config.Server.DataDir, "log")
	if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
		log.Error("failed to create directory :" + saveDir)
		return
	}

	path := filepath.Join(saveDir, fmt.Sprintf("task_%d.log", v.ID))
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Error("write task log failed :" + path)
		return
	}
}

func (t *Task) startedState() {
	t.Running = true
	log.Info(i18n.Localize("task.started"))
}

func (t *Task) completedState() {
	t.Running = false
	t.InstallingApp = nil
	log.Info(i18n.Localize("task.completed"))
}

func ScheduleRefreshApps() error {
	return instance.RunSchedule()
}

func RunInstallApp(v model.InstalledApp) {
	go instance.RunImmediately(v)
}

func GetCurrentInstallingApp() *model.InstalledApp {
	if instance.InstallingApp == nil {
		return nil
	}
	if !instance.Running {
		return nil
	}

	return instance.InstallingApp
}

func ReloadTask() error {
	if instance.c != nil {
		<-instance.c.Stop().Done()
		instance.c = nil
	}

	return instance.RunSchedule()
}
