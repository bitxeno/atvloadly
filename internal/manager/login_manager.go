package manager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	execx "github.com/bitxeno/atvloadly/internal/exec"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/gookit/event"
)

type LoginManager struct {
	outputStdout *outputWriter

	stdin io.WriteCloser

	cancel context.CancelFunc
	em     *event.Manager
}

func NewLoginManager() *LoginManager {
	em := event.NewManager("output", event.UsePathMode)
	return &LoginManager{
		outputStdout: newOutputWriter(em),

		em: em,
	}
}

func (t *LoginManager) Start(ctx context.Context, account, password string) error {
	t.outputStdout.Reset()

	// set execute timeout 10 minutes
	timeout := 10 * time.Minute
	ctx, cancel := context.WithTimeout(ctx, timeout)
	t.cancel = cancel

	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		log.Err(err).Msg("Error creating stdin pipe: ")
		return err
	}
	t.stdin = stdinWriter
	defer func() {
		_ = stdinReader.Close()
		_ = t.stdin.Close()
		t.stdin = nil
	}()

	err = execx.CommandContext(ctx, "plumesign", "account", "login", "-u", account, "-p", password).
		WithTimeout(timeout).
		WithDir(app.Config.Server.DataDir).
		WithEnv(GetRunEnvs()).
		WithStdout(t.outputStdout).
		WithStderr(t.outputStdout).
		WithStdin(stdinReader).
		Run()
	if err != nil {
		if errors.Is(err, execx.ErrCommandTimeout) {
			log.Err(err).Msgf("Login exceeded %d-minute timeout limit. %s", int(timeout.Minutes()), t.ErrorLog())
			return fmt.Errorf("Login exceeded %d-minute timeout limit. %s", int(timeout.Minutes()), err.Error())
		}
		log.Err(err).Msgf("Error executing login script. %s", t.ErrorLog())
	}
	return err
}

func (t *LoginManager) Close() {
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}
	if t.em != nil {
		_ = t.em.CloseWait()
	}
}

func (t *LoginManager) OnOutput(fn func(string)) {
	t.em.On("output", event.ListenerFunc(func(e event.Event) error {
		fn(e.Get("text").(string))
		return nil
	}))
}

func (t *LoginManager) Write(p []byte) {
	_, _ = t.stdin.Write(p)
}

func (t *LoginManager) ErrorLog() string {
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

func (t *LoginManager) OutputLog() string {
	return t.outputStdout.String()
}
