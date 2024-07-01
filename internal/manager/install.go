package manager

import (
	"context"
	"io"
	"os/exec"
	"path/filepath"
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
	// set execute timeout 5 miniutes
	timeout := 5 * time.Minute
	ctx, cancel := context.WithTimeout(ctx, timeout)
	t.cancel = cancel

	cmd := exec.CommandContext(ctx, "sideloader", "install", "--quiet", "--nocolor", "--udid", udid, "-a", account, "-p", password, ipaPath)
	if !t.quietMode {
		cmd = exec.CommandContext(ctx, "sideloader", "install", "--nocolor", "--udid", udid, "-a", account, "-p", password, ipaPath)
	}
	cmd.Dir = app.Config.Server.DataDir
	cmd.Env = []string{"SIDELOADER_CONFIG_DIR=" + app.SideloaderDataDir()}
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
		}
		log.Err(err).Msg("Error start installation script.")
		return err
	}

	err = cmd.Wait()
	if err != nil {
		log.Err(err).Msgf("Error executing installation script. %s", t.ErrorLog())
	}
	return err
}

func (t *InstallManager) Close() {
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}
	if t.em != nil {
		t.em.CloseWait()
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
