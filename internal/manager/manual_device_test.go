package manager

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bitxeno/atvloadly/internal/model"
)

func writeJSON(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadManualDevicesFromDir(t *testing.T) {
	dir := t.TempDir()
	// A manual device (pairing file has manual + remotepairing_ip).
	writeJSON(t, dir, "AAA.json", `{"manual":true,"model":"iPhone14,4","name":"Phone","remotepairing_ip":"100.65.100.37","remotepairing_udid":"AAA"}`)
	// An mDNS-discovered device's record (no manual / no ip): must be skipped.
	writeJSON(t, dir, "BBB.json", `{"model":"AppleTV11,1","name":"AppleTV","remotepairing_udid":"BBB"}`)

	dm := newDeviceManager()
	dm.loadManualDevicesFromDir(dir)

	devices := dm.GetDevices()
	if len(devices) != 1 {
		t.Fatalf("want 1 manual device, got %d: %+v", len(devices), devices)
	}
	d := devices[0]
	if !d.Manual || d.IP != "100.65.100.37" || d.UDID != "AAA" || d.Name != "Phone" {
		t.Fatalf("unexpected manual device: %+v", d)
	}
	if d.DeviceClass != string(model.DeviceClassiPhone) {
		t.Fatalf("device class = %q", d.DeviceClass)
	}
}

func TestManualDeviceDedupedByDiscovered(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "AAA.json", `{"manual":true,"model":"iPhone14,4","name":"Phone","remotepairing_ip":"100.65.100.37","remotepairing_udid":"AAA"}`)

	dm := newDeviceManager()
	dm.loadManualDevicesFromDir(dir)
	// Simulate mDNS discovering the same UDID with a fresher IP.
	dm.devices.Store("AAA", model.Device{ID: "AAA", UDID: "AAA", Name: "Phone", IP: "192.168.1.9", Status: model.Paired})

	devices := dm.GetDevices()
	if len(devices) != 1 {
		t.Fatalf("want deduped to 1 device, got %d", len(devices))
	}
	if devices[0].IP != "192.168.1.9" {
		t.Fatalf("discovered device must win, got IP %q", devices[0].IP)
	}
}

func TestDeleteManualDeviceRemovesBothFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	pdir := filepath.Join(dir, ".config", "PlumeImpactor", "pairing_files")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, pdir, "AAA.json", `{"manual":true,"remotepairing_ip":"100.65.100.37","remotepairing_udid":"AAA"}`)
	writeJSON(t, pdir, "AAA.plist", "plist")

	if err := DeleteManualDevice("AAA"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	for _, ext := range []string{".json", ".plist"} {
		if _, err := os.Stat(filepath.Join(pdir, "AAA"+ext)); !os.IsNotExist(err) {
			t.Fatalf("%s should be deleted", ext)
		}
	}
}

func withTunnelProbe(t *testing.T, fn func(context.Context, string, DiscoveryProgress) (uint16, *PairingServiceInfo, error)) {
	t.Helper()
	previous := discoverTunnelPortFunc
	discoverTunnelPortFunc = fn
	t.Cleanup(func() { discoverTunnelPortFunc = previous })
}

func withRememberedPortProbe(t *testing.T, fn func(context.Context, string, uint16) (*PairingServiceInfo, error)) {
	t.Helper()
	previous := probeRememberedPortFunc
	probeRememberedPortFunc = fn
	t.Cleanup(func() { probeRememberedPortFunc = previous })
}

func TestPrepareManualDeviceResolvesPort(t *testing.T) {
	calls := 0
	withTunnelProbe(t, func(ctx context.Context, ip string, _ DiscoveryProgress) (uint16, *PairingServiceInfo, error) {
		calls++
		if ip != "100.65.100.37" {
			t.Fatalf("unexpected ip: %s", ip)
		}
		return 49152, &PairingServiceInfo{Port: 49152, Tunnel: true}, nil
	})

	dm := newDeviceManager()
	dev := model.Device{ID: "id", IP: "100.65.100.37", UDID: "AAA", Manual: true}
	if err := dm.PrepareManualDevice(context.Background(), &dev); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if dev.Port != 49152 {
		t.Fatalf("port = %d, want 49152", dev.Port)
	}
	if calls != 1 {
		t.Fatalf("probe calls = %d, want 1", calls)
	}
}

func TestPrepareManualDeviceNoopForDiscovered(t *testing.T) {
	calls := 0
	withTunnelProbe(t, func(context.Context, string, DiscoveryProgress) (uint16, *PairingServiceInfo, error) {
		calls++
		return 1, nil, nil
	})
	dm := newDeviceManager()
	dev := model.Device{IP: "192.168.1.9", UDID: "BBB", Manual: false}
	if err := dm.PrepareManualDevice(context.Background(), &dev); err != nil {
		t.Fatal(err)
	}
	if calls != 0 || dev.Port != 0 {
		t.Fatalf("discovered device must not be probed: calls=%d port=%d", calls, dev.Port)
	}
}

func TestPrepareManualDeviceReusesFreshPort(t *testing.T) {
	calls := 0
	withTunnelProbe(t, func(context.Context, string, DiscoveryProgress) (uint16, *PairingServiceInfo, error) {
		calls++
		return 1, nil, nil
	})
	dm := newDeviceManager()
	dev := model.Device{IP: "100.65.100.37", UDID: "AAA", Manual: true, Port: 49152, PortResolvedAt: time.Now()}
	if err := dm.PrepareManualDevice(context.Background(), &dev); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("fresh cached port must not be re-probed, calls=%d", calls)
	}
}

func TestPrepareManualDeviceReprobesExpiredPort(t *testing.T) {
	calls := 0
	withTunnelProbe(t, func(context.Context, string, DiscoveryProgress) (uint16, *PairingServiceInfo, error) {
		calls++
		return 50000, nil, nil
	})
	// The remembered port is dead: the single-port re-check must fail so the
	// full discovery runs.
	withRememberedPortProbe(t, func(context.Context, string, uint16) (*PairingServiceInfo, error) {
		return nil, errors.New("stale port")
	})
	dm := newDeviceManager()
	dev := model.Device{IP: "100.65.100.37", UDID: "AAA", Manual: true, Port: 49152, PortResolvedAt: time.Now().Add(-2 * manualPortTTL)}
	if err := dm.PrepareManualDevice(context.Background(), &dev); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || dev.Port != 50000 {
		t.Fatalf("expired port must be re-probed: calls=%d port=%d", calls, dev.Port)
	}
}

func TestPrepareManualDeviceReusesAliveRememberedPort(t *testing.T) {
	// An expired port that still answers the handshake is reused without a
	// full-range scan.
	withTunnelProbe(t, func(context.Context, string, DiscoveryProgress) (uint16, *PairingServiceInfo, error) {
		t.Fatal("full discovery must not run when the remembered port is alive")
		return 0, nil, nil
	})
	withRememberedPortProbe(t, func(_ context.Context, _ string, port uint16) (*PairingServiceInfo, error) {
		if port != 49152 {
			t.Fatalf("unexpected remembered port %d", port)
		}
		return &PairingServiceInfo{Port: port, Tunnel: true}, nil
	})
	dm := newDeviceManager()
	dev := model.Device{IP: "100.65.100.37", UDID: "AAA", Manual: true, Port: 49152, PortResolvedAt: time.Now().Add(-2 * manualPortTTL)}
	if err := dm.PrepareManualDevice(context.Background(), &dev); err != nil {
		t.Fatal(err)
	}
	if dev.Port != 49152 || time.Since(dev.PortResolvedAt) >= manualPortTTL {
		t.Fatalf("alive remembered port must be refreshed: port=%d resolvedAt=%v", dev.Port, dev.PortResolvedAt)
	}
}

func TestPrepareManualDeviceErrorKeepsPortZero(t *testing.T) {
	withTunnelProbe(t, func(context.Context, string, DiscoveryProgress) (uint16, *PairingServiceInfo, error) {
		return 0, nil, ErrTunnelPortNotFound
	})
	dm := newDeviceManager()
	dev := model.Device{IP: "100.65.100.37", UDID: "AAA", Manual: true}
	if err := dm.PrepareManualDevice(context.Background(), &dev); err == nil {
		t.Fatal("expected error")
	}
	if dev.Port != 0 {
		t.Fatalf("port must stay 0 on error, got %d", dev.Port)
	}
}

func TestPrepareManualDeviceConcurrentSingleProbe(t *testing.T) {
	var calls int32
	withTunnelProbe(t, func(context.Context, string, DiscoveryProgress) (uint16, *PairingServiceInfo, error) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(20 * time.Millisecond)
		return 49152, nil, nil
	})
	dm := newDeviceManager()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dev := model.Device{IP: "100.65.100.37", UDID: "AAA", Manual: true}
			_ = dm.PrepareManualDevice(context.Background(), &dev)
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("concurrent prepare probed %d times, want 1", got)
	}
}
