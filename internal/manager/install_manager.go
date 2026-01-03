package manager

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/gookit/event"
)

type InstallManager struct {
	quietMode bool

	outputStdout *outputWriter
	outputStderr *outputWriter

	stdin io.WriteCloser

	cancel context.CancelFunc
	em     *event.Manager
}

func NewInstallManager() *InstallManager {
	em := event.NewManager("output", event.UsePathMode)
	return &InstallManager{
		quietMode:    true,
		outputStdout: newOutputWriter(em),
		outputStderr: newOutputWriter(em),

		em: em,
	}
}

func NewInteractiveInstallManager() *InstallManager {
	ins := NewInstallManager()
	ins.quietMode = false
	return ins
}

func (t *InstallManager) TryStart(ctx context.Context, udid, account, password, ipaPath string) error {
	err := t.Start(ctx, udid, account, password, ipaPath)
	if err != nil {
		// AppleTV system has reboot/lockdownd sleep, try restart usbmuxd to fix
		// LOCKDOWN_E_MUX_ERROR / AFC_E_MUX_ERROR /
		ipaName := filepath.Base(ipaPath)
		log.Infof("Try restarting usbmuxd to fix afc connect issue. %s", ipaName)
		if err = RestartUsbmuxd(); err == nil {
			log.Infof("Restart usbmuxd complete, try install ipa again. %s", ipaName)
			time.Sleep(5 * time.Second)
			err = t.Start(ctx, udid, account, password, ipaPath)
		}
	}
	return err
}

func (t *InstallManager) Start(ctx context.Context, udid, account, password, ipaPath string) error {
	t.outputStdout.Reset()
	t.outputStderr.Reset()

	// set execute timeout 30 miniutes
	timeout := 30 * time.Minute
	ctx, cancel := context.WithTimeout(ctx, timeout)
	t.cancel = cancel

	cmd := exec.CommandContext(ctx, "sideloader", "install", "--singlethread", "--quiet", "--nocolor", "--udid", udid, "-a", account, "-p", password, ipaPath)
	if !t.quietMode {
		cmd = exec.CommandContext(ctx, "sideloader", "install", "--singlethread", "--nocolor", "--udid", udid, "-a", account, "-p", password, ipaPath)
	}
	cmd.Dir = app.Config.Server.DataDir
	// 1. 初始化环境变量列表，保留程序核心需要的变量
	env := []string{"SIDELOADER_CONFIG_DIR=" + app.SideloaderDataDir()}

	// 2. 定义需要透传的代理变量白名单（包含大写和小写形式）
	proxyVars := []string{
		"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "ALL_PROXY",
		"http_proxy", "https_proxy", "no_proxy", "all_proxy",
	}

	// 3. 遍历系统环境变量，只追加匹配白名单的变量
	for _, e := range os.Environ() {
		for _, k := range proxyVars {
			// 匹配 "KEY=" 前缀，确保准确匹配变量名
			if strings.HasPrefix(e, k+"=") {
				env = append(env, e)
				break
			}
		}
	}

	cmd.Env = env
	cmd.Stdout = t.outputStdout
	cmd.Stderr = t.outputStderr

	var err error
	t.stdin, err = cmd.StdinPipe()
	if err != nil {
		log.Err(err).Msg("Error creating stdin pipe: ")
		return err
	}
	defer t.stdin.Close()

	if err := cmd.Start(); err != nil {
		if err == context.DeadlineExceeded {
			_ = cmd.Process.Kill()
			log.Err(err).Msgf("Installation exceeded %d-minute timeout limit. %s", int(timeout.Minutes()), t.ErrorLog())
			err = fmt.Errorf("Installation exceeded %d-minute timeout limit. %s", int(timeout.Minutes()), err.Error())
		}
		return err
	}

	err = cmd.Wait()
	if err != nil {
		log.Err(err).Msgf("Error executing installation script. %s", t.ErrorLog())
	}
	return err
}

func (t *InstallManager) CleanTempFiles(ipaPath string) {
	ipaName := filepath.Base(ipaPath)
	fileNameWithoutExt := strings.TrimSuffix(ipaName, filepath.Ext(ipaName))
	os.RemoveAll(filepath.Join(app.Config.Server.DataDir, "tmp", fileNameWithoutExt+".ipa"))
	os.RemoveAll(filepath.Join(app.Config.Server.DataDir, "tmp", fileNameWithoutExt+".png"))
	os.RemoveAll(filepath.Join(os.TempDir(), fileNameWithoutExt+".ipa"))
}

func (t *InstallManager) Close() {
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}
	if t.em != nil {
		_ = t.em.CloseWait()
	}
}

func (t *InstallManager) OnOutput(fn func(string)) {
	t.em.On("output", event.ListenerFunc(func(e event.Event) error {
		fn(e.Get("text").(string))
		return nil
	}))
}

func (t *InstallManager) Write(p []byte) {
	_, _ = t.stdin.Write(p)
}

func (t *InstallManager) ErrorLog() string {
	return t.outputStderr.String()
}

func (t *InstallManager) OutputLog() string {
	return t.outputStdout.String()
}

func (t *InstallManager) WriteLog(msg string) {
	_, _ = t.outputStdout.Write([]byte(msg))
}

func (t *InstallManager) SaveLog(id uint) {
	data := t.OutputLog() + t.ErrorLog()

	// Hide log password string
	// data = strings.Replace(data, v.Password, "******", -1)

	saveDir := filepath.Join(app.Config.Server.DataDir, "log")
	if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
		log.Error("failed to create directory :" + saveDir)
		return
	}

	path := filepath.Join(saveDir, fmt.Sprintf("task_%d.log", id))
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		log.Error("write log failed :" + path)
		return
	}
}

type outputWriter struct {
	data []byte
	em   *event.Manager
}

func newOutputWriter(em *event.Manager) *outputWriter {
	return &outputWriter{
		em: em,
	}
}

func (w *outputWriter) Write(p []byte) (n int, err error) {
	w.data = append(w.data, p...)
	w.em.MustFire("output", event.M{"text": string(p)})

	n = len(p)
	return n, nil
}

func (w *outputWriter) String() string {
	return string(w.data)
}

func (w *outputWriter) Reset() {
	w.data = []byte{}
}
