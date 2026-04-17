package manager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/exec"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
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

func (t *PairManager) Start(ctx context.Context, device model.Device) error {
	savePairingFile := filepath.Join(app.RemotePairingDir(), fmt.Sprintf("%s.plist", device.UDID))

	// Set execute timeout to 1 minute.
	timeout := time.Minute
	ctx, cancel := context.WithCancel(ctx)
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

	port := fmt.Sprintf("%d", device.Port)
	err = exec.CommandContext(ctx, "plumesign", "pair", "--ip", device.IP, "--port", port, "--output", savePairingFile).
		WithTimeout(timeout).
		WithDir(app.Config.Server.DataDir).
		WithEnv(GetRunEnvs()).
		WithStdout(t.outputStdout).
		WithStderr(t.outputStderr).
		WithStdin(stdinReader).
		Run()
	if err != nil {
		log.Err(err).Msgf("Error executing pair script. %s", t.ErrorLog())
		return err
	}

	log.Infof("Pairing successful for device %s (%s) pairing file saved at %s", device.Name, device.UDID, savePairingFile)
	return nil
}

func (t *PairManager) Close() {
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}
	if t.em != nil {
		_ = t.em.CloseWait()
	}
}

func (t *PairManager) OnOutput(fn func(string)) {
	t.em.On("output", event.ListenerFunc(func(e event.Event) error {
		fn(e.Get("text").(string))
		return nil
	}))
}

func (t *PairManager) Write(p []byte) {
	if t.stdin != nil {
		_, _ = t.stdin.Write(p)
	}
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

func ImportPairingFile(ip string, port string, data []byte, override bool) error {
	// Check if the current system is macOS, if so, importing is not supported
	if runtime.GOOS == "darwin" {
		return errors.New("importing pairing file is not supported on macOS")
	}

	udid, err := checkPairingFile(ip, port, data)
	if err != nil {
		return fmt.Errorf("pairing file validation failed: %w", err)
	}

	savePairingFile := filepath.Join(app.RemotePairingDir(), fmt.Sprintf("%s.plist", udid))
	if err := os.WriteFile(savePairingFile, data, 0600); err != nil {
		return fmt.Errorf("failed to save pairing file: %w", err)
	}

	log.Infof("Pairing file imported successfully: %s", udid)
	return nil
}

func checkPairingFile(ip string, port string, data []byte) (string, error) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "pairing-*.plist")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpFilePath := tmpFile.Name()

	// Ensure the temporary file is deleted when the function exits
	defer func() {
		if removeErr := os.Remove(tmpFilePath); removeErr != nil && !os.IsNotExist(removeErr) {
			log.Warnf("failed to remove temp pairing file %s: %v", tmpFilePath, removeErr)
		}
	}()

	// Write data to the temporary file
	if _, err := tmpFile.Write(data); err != nil {
		if closeErr := tmpFile.Close(); closeErr != nil {
			return "", fmt.Errorf("failed to close temp file after write error: %v", closeErr)
		}
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	// Execute the check command
	output, err := exec.NewCommand("plumesign", "check", "pairing", "--ip", ip, "--port", port, "-f", tmpFilePath).
		WithDir(app.Config.Server.DataDir).
		WithEnv(GetRunEnvs()).
		CombinedOutput()
	if err != nil {
		return "", err
	}

	// Parse UDID
	re := regexp.MustCompile("UniqueDeviceID: `([^`]+)`")
	matches := re.FindStringSubmatch(string(output))
	if len(matches) >= 2 {
		return matches[1], nil
	}

	return "", errors.New("UDID not found in output")
}
