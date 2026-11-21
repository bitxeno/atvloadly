package manager

import (
	"context"
	_ "embed"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
)

//go:embed restart_apple_tv.py
var restartAppleTVScript string

var postInstallRestarts = installRestarts{
	devices: make(map[string]*deviceInstalls),
	restart: restartAppleTV,
}

type deviceInstalls struct {
	active     int
	requested  bool
	restarting chan struct{}
}

type installRestarts struct {
	mu      sync.Mutex
	devices map[string]*deviceInstalls
	restart func(string) error
}

// BeginDeviceInstall reserves the device immediately, including queued work.
// If ready is non-nil, wait for it before starting I/O on the device.
// Call finish once, including when the reservation is cancelled.
func BeginDeviceInstall(udid string) (ready <-chan struct{}, finish func(bool)) {
	return postInstallRestarts.begin(udid)
}

func (r *installRestarts) begin(udid string) (<-chan struct{}, func(bool)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.devices[udid]
	if state == nil {
		state = &deviceInstalls{}
		r.devices[udid] = state
	}
	state.active++
	return state.restarting, func(request bool) { r.finish(udid, state, request) }
}

func (r *installRestarts) finish(udid string, state *deviceInstalls, request bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state.active--
	state.requested = state.requested || request
	if state.active > 0 || state.restarting != nil {
		return
	}
	if !state.requested {
		delete(r.devices, udid)
		return
	}
	state.requested = false
	state.restarting = make(chan struct{})
	go func() {
		if err := r.restart(udid); err != nil {
			log.Err(err).Msgf("Post-install Apple TV restart failed for %s", udid)
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		close(state.restarting)
		state.restarting = nil
		if state.active == 0 {
			delete(r.devices, udid)
		}
	}()
}

func restartAppleTV(udid string) error {
	dev, ok := GetDeviceByUDID(udid)
	if !ok {
		return fmt.Errorf("device is no longer connected")
	}
	if dev.DeviceClass != string(model.DeviceClassAppleTV) {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	args := []string{"-I", "-c", restartAppleTVScript, string(dev.Connection), udid}
	if dev.Connection == model.DeviceConnectionRemote {
		if err := PrepareManualDevice(ctx, dev); err != nil {
			return err
		}
		args = append(args, dev.IP, strconv.Itoa(int(dev.Port)), filepath.Join(app.RemotePairingDir(), udid+".plist"))
	}
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	output, err := exec.CommandContext(ctx, python, args...).CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	log.Infof("Requested post-install Apple TV restart for %s", udid)
	recovery, stop := context.WithTimeout(context.Background(), 3*time.Minute)
	defer stop()
	// The request was acknowledged before tvOS shut down. An AFC response
	// from the old session does not establish that a new install can start.
	err = waitForRestart(recovery, 2*time.Second, func() bool {
		return checkAfcServiceStatus(recovery, udid) == nil
	})
	if err != nil {
		return fmt.Errorf("restart acknowledged, device reconnect unconfirmed: %w", err)
	}
	log.Infof("Apple TV %s is reachable after restarting", udid)
	return nil
}

func waitForRestart(ctx context.Context, interval time.Duration, reachable func() bool) error {
	offline := false
	for ctx.Err() == nil {
		if !reachable() {
			offline = true
		} else if offline {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
	return ctx.Err()
}
