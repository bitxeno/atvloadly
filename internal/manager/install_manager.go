package manager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/gookit/event"
)

var ErrAccountInvalid = errors.New("account invalid")

type InstallManager struct {
	quietMode bool

	outputStdout *outputWriter

	stdin io.WriteCloser

	cancel              context.CancelFunc
	em                  *event.Manager
	ProvisioningProfile *model.MobileProvisioningProfile
}

type InstallOptions struct {
	UDID             string
	Account          string
	Password         string
	IpaPath          string
	RemoveExtensions bool
	RefreshMode      bool
}

func NewInstallManager() *InstallManager {
	em := event.NewManager("output", event.UsePathMode)
	return &InstallManager{
		quietMode:    true,
		outputStdout: newOutputWriter(em),

		em: em,
	}
}

func NewInteractiveInstallManager() *InstallManager {
	ins := NewInstallManager()
	ins.quietMode = false
	return ins
}

func (t *InstallManager) TryStart(ctx context.Context, opts InstallOptions) error {
	err := t.Start(ctx, opts)
	if err != nil {
		if t.IsAccountInvalid() {
			return fmt.Errorf("%s %s %w", t.ErrorLog(), err.Error(), ErrAccountInvalid)
		}

		// AppleTV system has reboot/lockdownd sleep, try restart usbmuxd to fix
		// LOCKDOWN_E_MUX_ERROR / AFC_E_MUX_ERROR /
		ipaName := filepath.Base(opts.IpaPath)
		log.Infof("Try restarting usbmuxd to fix afc connect issue. %s", ipaName)
		if errmux := RestartUsbmuxd(); errmux == nil {
			// iPhone reconnect may take a while, wait some time
			time.Sleep(30 * time.Second)
			log.Infof("Restart usbmuxd complete, try install ipa again. %s", ipaName)
			err = t.Start(ctx, opts)
		}
	}
	return err
}

func (t *InstallManager) Start(ctx context.Context, opts InstallOptions) error {
	t.outputStdout.Reset()

	// set execute timeout 30 miniutes
	timeout := 30 * time.Minute
	ctx, cancel := context.WithTimeout(ctx, timeout)
	t.cancel = cancel

	provisionPath := t.GetMobileProvisionPath()
	defer func() {
		if _, err := os.Stat(provisionPath); err == nil {
			_ = os.Remove(provisionPath)
		}
	}()

	if err := CheckAfcServiceStatus(opts.UDID); err != nil {
		return fmt.Errorf("afc service not available: %w", err)
	}

	args := []string{"sign", "--apple-id", "--register-and-install", "--output-provision", provisionPath, "--udid", opts.UDID, "-u", opts.Account, "-p", opts.IpaPath}
	if opts.RemoveExtensions {
		args = append(args, "--remove-extensions")
	}
	if opts.RefreshMode {
		args = append(args, "--refresh")
	}
	cmd := exec.CommandContext(ctx, "plumesign", args...)
	cmd.Dir = app.Config.Server.DataDir
	cmd.Env = GetRunEnvs()
	cmd.Stdout = t.outputStdout
	cmd.Stderr = t.outputStdout

	var err error
	t.stdin, err = cmd.StdinPipe()
	if err != nil {
		log.Err(err).Msg("Error creating stdin pipe: ")
		return err
	}
	defer func() {
		_ = t.stdin.Close()
	}()

	if err := cmd.Start(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			_ = cmd.Process.Kill()
			log.Err(err).Msgf("Installation exceeded %d-minute timeout limit. %s", int(timeout.Minutes()), t.ErrorLog())
			err = fmt.Errorf("Installation exceeded %d-minute timeout limit. %s", int(timeout.Minutes()), err.Error())
		}
		return err
	}

	if err = cmd.Wait(); err != nil {
		return err
	}

	if provisionProfile, perr := model.ParseMobileProvisioningProfileFile(provisionPath); perr == nil {
		t.ProvisioningProfile = provisionProfile
	}

	return nil
}

func (t *InstallManager) GetMobileProvisionPath() string {
	return path.Join(os.TempDir(), fmt.Sprintf("embedded.mobileprovision.%d", time.Now().UnixNano()))
}

func (t *InstallManager) CleanTempFiles(ipaPath string) {
	ipaName := filepath.Base(ipaPath)
	fileNameWithoutExt := strings.TrimSuffix(ipaName, filepath.Ext(ipaName))
	_ = os.RemoveAll(filepath.Join(app.Config.Server.DataDir, "tmp", fileNameWithoutExt+".ipa"))
	_ = os.RemoveAll(filepath.Join(app.Config.Server.DataDir, "tmp", fileNameWithoutExt+".png"))
	_ = os.RemoveAll(filepath.Join(os.TempDir(), fileNameWithoutExt+".ipa"))

	pat := filepath.Join(os.TempDir(), "plume_stage*")
	matches, _ := filepath.Glob(pat)
	for _, m := range matches {
		_ = os.RemoveAll(m)
	}
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
	if t.stdin != nil {
		_, _ = t.stdin.Write(p)
	}
}

func (t *InstallManager) ErrorLog() string {
	data := t.outputStdout.String()
	if data == "" {
		return ""
	}

	var lines []string
	for _, l := range strings.Split(data, "\n") {
		if strings.HasPrefix(strings.ToLower(l), "error") {
			lines = append(lines, l)
		}
	}
	return strings.Join(lines, "\n")
}

func (t *InstallManager) IsAccountInvalid() bool {
	log := t.OutputLog()
	return strings.Contains(log, "plumesign account list") || strings.Contains(log, "Can't log-in") || strings.Contains(log, "DeveloperSession creation failed")
}

func (t *InstallManager) IsSuccess() bool {
	log := t.OutputLog()
	return strings.Contains(log, "Installation Succeeded") || strings.Contains(log, "Installation complete")
}

func (t *InstallManager) OutputLog() string {
	return t.outputStdout.String()
}

func (t *InstallManager) WriteLog(msg string) {
	_, _ = t.outputStdout.Write([]byte(msg))
}

func (t *InstallManager) SaveLog(id uint) {
	data := t.OutputLog()

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
