package manager

import (
	"context"
	"io"
	"os/exec"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/gookit/event"
)

type PairManager struct {
	outputStdout *pairOutputWriter
	outputStderr *pairOutputWriter

	stdin io.WriteCloser

	cancel context.CancelFunc
	em     *event.Manager
}

func NewPairManager() *PairManager {
	em := event.NewManager("output", event.UsePathMode)
	return &PairManager{
		outputStdout: newPairOutputWriter(em),
		outputStderr: newPairOutputWriter(em),

		em: em,
	}
}

func (t *PairManager) Start(ctx context.Context, udid string) error {
	// set execute timeout 1 miniutes
	timeout := time.Minute
	ctx, cancel := context.WithTimeout(ctx, timeout)
	t.cancel = cancel

	cmd := exec.CommandContext(ctx, "idevicepair", "pair", "-u", udid, "-w")
	cmd.Dir = app.Config.Server.DataDir
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
		log.Err(err).Msg("Error start pair script.")
		return err
	}

	err = cmd.Wait()
	if err != nil {
		log.Err(err).Msgf("Error executing pair script. %s", t.ErrorLog())
	}
	return err
}

func (t *PairManager) Close() {
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}
	if t.em != nil {
		t.em.CloseWait()
	}
}

func (t *PairManager) OnOutput(fn func(string)) {
	t.em.On("output", event.ListenerFunc(func(e event.Event) error {
		fn(e.Get("text").(string))
		return nil
	}))
}

func (t *PairManager) Write(p []byte) {
	_, _ = t.stdin.Write(p)
}

func (t *PairManager) ErrorLog() string {
	return t.outputStderr.String()
}

func (t *PairManager) OutputLog() string {
	return t.outputStdout.String()
}

type pairOutputWriter struct {
	data []byte
	em   *event.Manager
}

func newPairOutputWriter(em *event.Manager) *pairOutputWriter {
	return &pairOutputWriter{
		em: em,
	}
}

func (w *pairOutputWriter) Write(p []byte) (n int, err error) {
	w.data = append(w.data, p...)
	w.em.MustFire("output", event.M{"text": string(p)})

	n = len(p)
	return n, nil
}

func (w *pairOutputWriter) String() string {
	return string(w.data)
}
