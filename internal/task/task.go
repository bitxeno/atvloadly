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
		log.Info("app刷新定时任务未启用")
		return nil
	}

	t.c = cron.New()
	if _, err := t.c.AddFunc(app.Settings.Task.CrodTime, t.Run); err != nil {
		log.Err(err).Msgf("app刷新定时任务启动失败，定时格式错误：%s", app.Settings.Task.CrodTime)
		t.c = nil
		return err
	}

	log.Infof("app刷新定时任务已启动，时间: %s", app.Settings.Task.CrodTime)
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
		log.Err(err).Msg("获取安装列表失败")
		return
	}

	now := time.Now()
	failedList := []model.InstalledApp{}
	failedMsg := ""
	for _, v := range installedApps {
		if !t.checkNeedRefresh(v) {
			continue
		}

		log.Infof("开始执行安装ipa：%s", v.IpaName)
		if err := t.runInternal(v); err != nil {
			// AppleTV system has reboot/lockdownd sleep, try restart usbmuxd to fix
			if strings.Contains(err.Error(), "LOCKDOWN_E_MUX_ERROR") {
				log.Infof("尝试重启 usbmuxd 修复 LOCKDOWN_E_MUX_ERROR 错误. %s", v.IpaName)
				if err = manager.RestartUsbmuxd(); err == nil {
					log.Infof("usbmuxd 重启完成，再次尝试安装ipa：%s", v.IpaName)
					time.Sleep(5 * time.Second)
					err = t.runInternal(v)
				}
			}
		}
		if err != nil {
			now := time.Now()
			v.RefreshedDate = &now
			v.RefreshedResult = false
			_ = service.UpdateAppRefreshResult(v)

			failedList = append(failedList, v)
			failedMsg += fmt.Sprintf("app: %s\n 错误日志：%s\n\n", v.IpaName, err.Error())
		} else {
			v.RefreshedDate = &now
			v.RefreshedResult = true
			_ = service.UpdateAppRefreshResult(v)
		}
		log.Infof("安装ipa执行完成.任务：%s", v.IpaName)

		// 下一个执行延迟10秒
		time.Sleep(10 * time.Second)
	}

	// 发送安装失败通知
	if len(failedList) > 0 {
		_ = notify.Send("atvloadly 自动刷新任务执行失败", failedMsg)
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
	err := t.runInternal(v)
	if err == nil {
		v.RefreshedDate = &now
		v.RefreshedResult = true
		_ = service.UpdateAppRefreshResult(v)
	} else {
		v.RefreshedDate = &now
		v.RefreshedResult = false
		_ = service.UpdateAppRefreshResult(v)

		// 发送安装失败通知
		_ = notify.Send(fmt.Sprintf("[%s]刷新任务执行失败", v.IpaName), fmt.Sprintf("帐号：%s\n错误日志：%s", v.Account, err.Error()))
	}
}

func (t *Task) runInternal(v model.InstalledApp) error {
	t.InstallingApp = &v

	if v.Account == "" || v.Password == "" || v.UDID == "" {
		log.Info("任务帐号，密码，UDID为空")
		return fmt.Errorf("任务帐号，密码，UDID为空")
	}

	// 检查developer disk image是否已mounted
	// imageInfo, err := manager.GetDeviceMountImageInfo(v.UDID)
	// if err != nil {
	// 	log.Err(err).Msg("Check DeveloperDiskImage mounted error: ")
	// 	return err
	// }

	// if !imageInfo.ImageMounted {
	// 	log.Error("DeveloperDiskImage not mounted.")
	// 	return err
	// }

	// sideloader 会处理特殊字符"$"，对于带有这特殊字符的，需要加单引号包括
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

			// 处理中途需要输入才能继续的，如 Installing AltStore with Multiple AltServers Not Supported 消息
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

			// 处理中途需要输入才能继续的，如 Installing AltStore with Multiple AltServers Not Supported 消息
			if strings.Contains(lineText, "Press any key to continue") {
				_, _ = stdin.Write([]byte("\n"))
			}
		}
	}(readerErr)
	if err := cmd.Start(); nil != err {
		data := []byte(output.String())
		t.writeLog(v, data)
		log.Err(err).Msg("执行安装脚本出错")
		return fmt.Errorf(outputErr.String(), err)
	}

	err = cmd.Wait()
	if err != nil {
		data := []byte(output.String())
		t.writeLog(v, data)
		log.Err(err).Msg("执行安装脚本出错")
		return fmt.Errorf(outputErr.String(), err)
	}

	data := []byte(output.String())
	t.writeLog(v, data)
	if strings.Contains(string(data), "Installation Succeeded") {
		log.Info("执行安装脚本成功")
		return nil
	} else {
		log.Info("执行安装脚本失败")
		return fmt.Errorf(outputErr.String())
	}
}

func (t *Task) writeLog(v model.InstalledApp, data []byte) {
	// 打码密码字符串
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
	log.Info("开始执行定时任务...")
}

func (t *Task) completedState() {
	t.Running = false
	t.InstallingApp = nil
	log.Warn("定时任务执行完成")
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
