package manager

import (
	"context"
	"testing"
	"time"

	"github.com/bitxeno/atvloadly/internal/model"
)

func withProbeDial(t *testing.T, fn func(context.Context, string, uint16, time.Duration) bool) {
	t.Helper()
	prev := probeDialFunc
	probeDialFunc = fn
	t.Cleanup(func() { probeDialFunc = prev })
}

// setProbeLastProbe forces a device's last-probe time so the min-age gate is
// bypassed (or skipped) in tests.
func setProbeLastProbe(dm *DeviceManager, key string, at time.Time) {
	dm.probeMu.Lock()
	defer dm.probeMu.Unlock()
	st, ok := dm.probeStates[key]
	if !ok {
		st = &deviceProbeState{}
		dm.probeStates[key] = st
	}
	st.lastProbe = at
}

func remoteDevice(udid, ip string, port uint16) model.Device {
	return model.Device{
		ID: udid, UDID: udid, IP: ip, Port: port,
		Connection: model.DeviceConnectionRemote, Status: model.Paired, Manual: false,
	}
}

func TestProbeOnceOnlyProbesDualSourceRemoteDevices(t *testing.T) {
	dm := newDeviceManager()
	dm.manualDevices.Store("m1", model.Device{UDID: "AAA", Manual: true, IP: "100.65.100.37"})

	// Should be probed: remote + paired + port + has manual record.
	dm.devices.Store("AAA", remoteDevice("AAA", "192.168.1.10", 49152))
	// Skipped: lockdown (port 0).
	dm.devices.Store("LOCK", model.Device{ID: "LOCK", UDID: "LOCK", IP: "192.168.1.11", Connection: model.DeviceConnectionLockdown, Status: model.Paired})
	// Skipped: pairable.
	pairable := remoteDevice("PAIR", "192.168.1.12", 49152)
	pairable.Status = model.Pairable
	dm.devices.Store("PAIR", pairable)
	// Skipped: no manual record.
	dm.devices.Store("CCC", remoteDevice("CCC", "192.168.1.13", 49152))

	var probed []string
	withProbeDial(t, func(_ context.Context, ip string, _ uint16, _ time.Duration) bool {
		probed = append(probed, ip)
		return true
	})

	dm.probeOnce(context.Background())

	if len(probed) != 1 || probed[0] != "192.168.1.10" {
		t.Fatalf("expected only the dual-source remote device to be probed, got %v", probed)
	}
}

func TestProbeMinAgeSkipsRecentlyAnnounced(t *testing.T) {
	dm := newDeviceManager()
	dm.manualDevices.Store("m1", model.Device{UDID: "AAA", Manual: true})
	dev := remoteDevice("AAA", "192.168.1.10", 49152)
	dm.devices.Store("AAA", dev)
	dm.markDeviceAnnounced(dev) // counts as a fresh probe

	calls := 0
	withProbeDial(t, func(context.Context, string, uint16, time.Duration) bool { calls++; return true })

	dm.probeOnce(context.Background())
	if calls != 0 {
		t.Fatalf("recently announced device must not be probed, calls=%d", calls)
	}
}

func TestProbeStaleAfterThresholdThenRecovers(t *testing.T) {
	dm := newDeviceManager()
	dm.manualDevices.Store("m1", model.Device{UDID: "AAA", Manual: true, IP: "100.65.100.37"})
	dev := remoteDevice("AAA", "192.168.1.10", 49152)
	dm.devices.Store("AAA", dev)
	key := probeKey(dev)

	withProbeDial(t, func(context.Context, string, uint16, time.Duration) bool { return false })

	for i := 0; i < probeFailThreshold; i++ {
		setProbeLastProbe(dm, key, time.Time{})
		dm.probeDevice(context.Background(), key, dev)
	}

	if !dm.isDeviceStale(dev) {
		t.Fatalf("device should be stale after %d failures", probeFailThreshold)
	}
	// GetDevices hides the stale mDNS entry and surfaces the manual one.
	got := dm.GetDevices()
	if len(got) != 1 || !got[0].Manual || got[0].IP != "100.65.100.37" {
		t.Fatalf("expected manual device to surface, got %+v", got)
	}

	// Recovery: a successful probe clears stale and mDNS wins again.
	withProbeDial(t, func(context.Context, string, uint16, time.Duration) bool { return true })
	setProbeLastProbe(dm, key, time.Time{})
	dm.probeDevice(context.Background(), key, dev)

	if dm.isDeviceStale(dev) {
		t.Fatal("stale flag should clear after a successful probe")
	}
	got = dm.GetDevices()
	if len(got) != 1 || got[0].Manual || got[0].IP != "192.168.1.10" {
		t.Fatalf("expected mDNS device to return, got %+v", got)
	}
}

func TestMarkDeviceAnnouncedResetsFailures(t *testing.T) {
	dm := newDeviceManager()
	dev := remoteDevice("AAA", "192.168.1.10", 49152)
	key := probeKey(dev)

	dm.probeMu.Lock()
	dm.probeStates[key] = &deviceProbeState{failures: 3, stale: true, lastProbe: time.Now().Add(-time.Hour)}
	dm.probeMu.Unlock()

	dm.markDeviceAnnounced(dev)

	dm.probeMu.Lock()
	st := dm.probeStates[key]
	dm.probeMu.Unlock()
	if st.failures != 0 || st.stale {
		t.Fatalf("announcement must reset failures/stale, got %+v", st)
	}
}
