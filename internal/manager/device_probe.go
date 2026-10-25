package manager

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
)

// Liveness probing for mDNS-discovered devices that also have a manual
// (pairing-file) record.
//
// A device that leaves the network without sending an mDNS goodbye stays in
// the discovery map forever, shadowing the manual entry. A background probe
// marks such entries stale (hidden from GetDevices) after repeated TCP
// failures, so the manual address takes over; a successful probe or a fresh
// mDNS announcement clears the stale flag again.
const (
	// probeInterval is how often the liveness loop runs.
	probeInterval = time.Minute
	// probeMinAge skips devices probed less than this long ago; a fresh mDNS
	// announcement counts as a probe, so just-added devices are not re-probed.
	probeMinAge = time.Minute
	// probeFailThreshold is the number of consecutive failures that marks a
	// device stale.
	probeFailThreshold = 5
	// probeDialTimeout bounds one TCP dial.
	probeDialTimeout = 2 * time.Second
)

// deviceProbeState tracks one device's liveness. Guarded by DeviceManager.probeMu.
type deviceProbeState struct {
	lastProbe time.Time
	failures  int
	stale     bool
}

// probeDialFunc dials a device's tunnel port; replaceable in tests.
var probeDialFunc = func(ctx context.Context, ip string, port uint16, timeout time.Duration) bool {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(int(port))))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// probeKey identifies a device for liveness tracking.
func probeKey(dev model.Device) string {
	if dev.UDID != "" {
		return dev.UDID
	}
	return dev.ServiceName
}

// markDeviceAnnounced records a successful "announcement" (mDNS Add / address
// refresh): it counts as a probe and clears any stale/failure state.
func (dm *DeviceManager) markDeviceAnnounced(dev model.Device) {
	key := probeKey(dev)
	if key == "" {
		return
	}
	dm.probeMu.Lock()
	defer dm.probeMu.Unlock()
	st, ok := dm.probeStates[key]
	if !ok {
		st = &deviceProbeState{}
		dm.probeStates[key] = st
	}
	st.lastProbe = time.Now()
	st.failures = 0
	st.stale = false
}

// clearProbeState drops liveness state for a removed device.
func (dm *DeviceManager) clearProbeState(key string) {
	if key == "" {
		return
	}
	dm.probeMu.Lock()
	delete(dm.probeStates, key)
	dm.probeMu.Unlock()
}

// isDeviceStale reports whether a device is currently marked stale.
func (dm *DeviceManager) isDeviceStale(dev model.Device) bool {
	key := probeKey(dev)
	if key == "" {
		return false
	}
	dm.probeMu.Lock()
	defer dm.probeMu.Unlock()
	st, ok := dm.probeStates[key]
	return ok && st.stale
}

// startLivenessProbe launches the background probe loop bound to ctx.
func (dm *DeviceManager) startLivenessProbe(ctx context.Context) {
	go dm.runLivenessProbe(ctx)
}

func (dm *DeviceManager) runLivenessProbe(ctx context.Context) {
	ticker := time.NewTicker(probeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dm.probeOnce(ctx)
		}
	}
}

// probeOnce probes every mDNS device that also has a manual record and is due
// for a check. Devices without a manual fallback are left untouched (a stale
// mDNS entry there is a separate concern).
func (dm *DeviceManager) probeOnce(ctx context.Context) {
	manualUDIDs := map[string]bool{}
	dm.manualDevices.Range(func(k, v any) bool {
		if udid := v.(model.Device).UDID; udid != "" {
			manualUDIDs[udid] = true
		}
		return true
	})
	if len(manualUDIDs) == 0 {
		return
	}

	type target struct {
		key string
		dev model.Device
	}
	targets := []target{}
	dm.devices.Range(func(k, v any) bool {
		dev := v.(model.Device)
		// Only remote-paired devices with a real tunnel port can be dialed;
		// lockdown/USB and transient pairable entries must never be probed.
		if dev.Connection != model.DeviceConnectionRemote || dev.Status != model.Paired || dev.Port == 0 {
			return true
		}
		if dev.UDID == "" || !manualUDIDs[dev.UDID] {
			return true
		}
		targets = append(targets, target{key: probeKey(dev), dev: dev})
		return true
	})

	for _, t := range targets {
		if ctx.Err() != nil {
			return
		}
		dm.probeDevice(ctx, t.key, t.dev)
	}
}

// probeDevice probes one device unless it was checked too recently, then
// updates its failure count and stale flag.
func (dm *DeviceManager) probeDevice(ctx context.Context, key string, dev model.Device) {
	dm.probeMu.Lock()
	st, ok := dm.probeStates[key]
	if !ok {
		st = &deviceProbeState{}
		dm.probeStates[key] = st
	}
	if !st.lastProbe.IsZero() && time.Since(st.lastProbe) < probeMinAge {
		dm.probeMu.Unlock()
		return
	}
	st.lastProbe = time.Now()
	dm.probeMu.Unlock()

	okDial := probeDialFunc(ctx, dev.IP, dev.Port, probeDialTimeout)

	dm.probeMu.Lock()
	defer dm.probeMu.Unlock()
	if okDial {
		st.failures = 0
		st.stale = false
		return
	}
	st.failures++
	if st.failures >= probeFailThreshold && !st.stale {
		st.stale = true
		log.Warnf("Device %s (%s) marked stale after %d failed probes; falling back to manual record", dev.Name, dev.IP, st.failures)
	}
}
