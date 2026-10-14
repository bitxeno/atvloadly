package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	execx "github.com/bitxeno/atvloadly/internal/exec"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/gookit/event"
)

type LoginManager struct {
	// Serializes attempts on this WebSocket to protect the shared output buffer.
	runMu sync.Mutex
	// Protects the currently active login subprocess and its input pipe.
	stateMu sync.Mutex

	outputStdout *outputWriter

	stdin *os.File

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
	if !t.runMu.TryLock() {
		return errors.New("login already in progress")
	}
	defer t.runMu.Unlock()
	t.outputStdout.Reset()

	// set execute timeout 10 minutes
	timeout := 10 * time.Minute
	ctx, cancel := context.WithTimeout(ctx, timeout)
	t.stateMu.Lock()
	t.cancel = cancel
	t.stateMu.Unlock()
	defer func() {
		t.stateMu.Lock()
		t.cancel = nil
		t.stateMu.Unlock()
		cancel()
	}()

	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		log.Err(err).Msg("Error creating stdin pipe: ")
		return err
	}
	t.stateMu.Lock()
	t.stdin = stdinWriter
	t.stateMu.Unlock()
	defer func() {
		// Remove the active reference before closing the specific pipe for
		// this attempt. In-flight os.File writes return an error on close.
		t.stateMu.Lock()
		if t.stdin == stdinWriter {
			t.stdin = nil
		}
		t.stateMu.Unlock()
		_ = stdinWriter.Close()
		_ = stdinReader.Close()
	}()

	err = execx.CommandContext(ctx, "plumesign", "account", "login", "-u", account, "-p", password).
		WithTimeout(timeout).
		WithDir(app.Config.Server.DataDir).
		WithEnv(GetRunEnvs()).
		WithStdout(t.outputStdout).
		WithStderr(t.outputStdout).
		WithStdin(stdinReader).
		WithSecret(password).
		Run()
	if err != nil {
		if errors.Is(err, execx.ErrCommandTimeout) {
			log.Err(err).Msgf("login timeout exceeded after %d minutes. %s", int(timeout.Minutes()), t.ErrorLog())
			return fmt.Errorf("login timeout exceeded after %d minutes: %s", int(timeout.Minutes()), err.Error()) //nolint:ST1005
		}
		log.Err(err).Msgf("Error executing login script. %s", t.ErrorLog())
	}
	return err
}

func (t *LoginManager) Close() {
	t.stateMu.Lock()
	cancel := t.cancel
	t.cancel = nil
	t.stateMu.Unlock()
	if cancel != nil {
		cancel()
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
	t.stateMu.Lock()
	writer := t.stdin
	t.stateMu.Unlock()
	// A late 2FA reply is harmless after the subprocess exits. Never keep
	// stateMu across Write: an OS pipe can block until its reader exits.
	if writer != nil {
		_, _ = writer.Write(p)
	}
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
