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

// BeginDeviceInstall reserves the device from queue admission through completion.
// Call the returned function exactly once, including for rejected or failed work.
func BeginDeviceInstall(udid string) func(bool) {
	return postInstallRestarts.begin(udid)
}

func (r *installRestarts) begin(udid string) func(bool) {
	r.mu.Lock()
	for {
		state := r.devices[udid]
		if state == nil {
			state = &deviceInstalls{}
			r.devices[udid] = state
		}
		if state.restarting == nil {
			state.active++
			r.mu.Unlock()
			return func(request bool) { r.finish(udid, state, request) }
		}
		done := state.restarting
		r.mu.Unlock()
		<-done
		r.mu.Lock()
	}
}

func (r *installRestarts) finish(udid string, state *deviceInstalls, request bool) {
	r.mu.Lock()
	state.active--
	state.requested = state.requested || request
	if state.active > 0 {
		r.mu.Unlock()
		return
	}
	if !state.requested {
		delete(r.devices, udid)
		r.mu.Unlock()
		return
	}
	state.restarting = make(chan struct{})
	r.mu.Unlock()

	if err := r.restart(udid); err != nil {
		log.Err(err).Msgf("Post-install Apple TV restart failed for %s", udid)
	}

	r.mu.Lock()
	delete(r.devices, udid)
	close(state.restarting)
	r.mu.Unlock()
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
	return nil
}
