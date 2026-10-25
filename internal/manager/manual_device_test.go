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

// When the remembered port does not answer and the address itself is
// unreachable (Tailscale client not connected), the full scan reports
// ErrDeviceUnreachable and PrepareManualDevice surfaces it, rather than
// swallowing it into a generic "port not found".
func TestPrepareManualDeviceReportsUnreachable(t *testing.T) {
	withTunnelProbe(t, func(context.Context, string, DiscoveryProgress) (uint16, *PairingServiceInfo, error) {
		return 0, nil, ErrDeviceUnreachable
	})
	withRememberedPortProbe(t, func(context.Context, string, uint16) (*PairingServiceInfo, error) {
		return nil, errors.New("no answer")
	})

	dm := newDeviceManager()
	dev := model.Device{IP: "100.65.100.37", UDID: "AAA", Manual: true, Port: 49152, PortResolvedAt: time.Now().Add(-2 * manualPortTTL)}
	err := dm.PrepareManualDevice(context.Background(), &dev)
	if !errors.Is(err, ErrDeviceUnreachable) {
		t.Fatalf("want ErrDeviceUnreachable, got %v", err)
	}
}

// A transient failure on the remembered-port re-check must be retried before
// escalating to a full-range scan, so one dropped handshake on a slow relay
// does not cost a multi-second sweep.
func TestProbeRememberedPortRetriesTransientFailure(t *testing.T) {
	attempts := 0
	withRememberedPortProbe(t, func(context.Context, string, uint16) (*PairingServiceInfo, error) {
		attempts++
		if attempts < rememberedProbeAttempts {
			return nil, errors.New("transient handshake timeout")
		}
		return &PairingServiceInfo{Port: 49152, Tunnel: true}, nil
	})

	si, err := probeRememberedPort(context.Background(), "100.65.100.37", 49152)
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if si == nil || !si.Tunnel {
		t.Fatalf("unexpected info: %+v", si)
	}
	if attempts != rememberedProbeAttempts {
		t.Fatalf("attempts = %d, want %d", attempts, rememberedProbeAttempts)
	}
}

// A remembered port that answers but is not a tunnel service must exhaust the
// retries and report failure, letting the caller fall back to a scan.
func TestProbeRememberedPortFailsWhenNotTunnel(t *testing.T) {
	attempts := 0
	withRememberedPortProbe(t, func(context.Context, string, uint16) (*PairingServiceInfo, error) {
		attempts++
		return &PairingServiceInfo{Port: 49152, Tunnel: false}, nil
	})

	if _, err := probeRememberedPort(context.Background(), "100.65.100.37", 49152); err == nil {
		t.Fatal("expected failure when the port is not a tunnel service")
	}
	if attempts != rememberedProbeAttempts {
		t.Fatalf("attempts = %d, want %d", attempts, rememberedProbeAttempts)
	}
}

// An already-cancelled context must not keep retrying.
func TestProbeRememberedPortHonorsContext(t *testing.T) {
	attempts := 0
	withRememberedPortProbe(t, func(context.Context, string, uint16) (*PairingServiceInfo, error) {
		attempts++
		return nil, errors.New("timeout")
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := probeRememberedPort(ctx, "100.65.100.37", 49152); err == nil {
		t.Fatal("expected error on cancelled context")
	}
	if attempts > 1 {
		t.Fatalf("cancelled context retried: attempts=%d", attempts)
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

func TestIsManualDeviceAddr(t *testing.T) {
	cases := []struct {
		ip string
		ok bool
	}{
		{"192.168.1.9", true},     // RFC 1918
		{"10.0.0.5", true},        // RFC 1918
		{"172.16.4.4", true},      // RFC 1918
		{"100.64.0.1", true},      // CGNAT (Tailscale)
		{"100.65.100.37", true},   // CGNAT (Tailscale)
		{"100.127.255.254", true}, // CGNAT upper bound
		{"8.8.8.8", false},        // public
		{"100.128.0.1", false},    // just outside CGNAT
		{"127.0.0.1", false},      // loopback
		{"::1", false},            // IPv6
		{"not-an-ip", false},      // garbage
		{"", false},               // empty
	}
	for _, c := range cases {
		if got := IsManualDeviceAddr(c.ip); got != c.ok {
			t.Errorf("IsManualDeviceAddr(%q) = %v, want %v", c.ip, got, c.ok)
		}
	}
}

func TestWarmManualDevicePortResolves(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	pdir := filepath.Join(dir, ".config", "PlumeImpactor", "pairing_files")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, pdir, "AAA.json", `{"manual":true,"model":"AppleTV11,1","name":"AppleTV","remotepairing_ip":"100.65.100.37","remotepairing_udid":"AAA"}`)

	calls := 0
	withTunnelProbe(t, func(context.Context, string, DiscoveryProgress) (uint16, *PairingServiceInfo, error) {
		calls++
		return 60001, &PairingServiceInfo{Port: 60001, Tunnel: true}, nil
	})

	dm := newDeviceManager()
	dm.WarmManualDevicePort("100.65.100.37")

	if calls != 1 {
		t.Fatalf("warm-up probed %d times, want 1", calls)
	}
	devices := dm.GetDevices()
	if len(devices) != 1 || devices[0].Port != 60001 {
		t.Fatalf("warm-up did not record the port: %+v", devices)
	}
}

func TestWarmManualDevicePortSkipsUnknownIP(t *testing.T) {
	withTunnelProbe(t, func(context.Context, string, DiscoveryProgress) (uint16, *PairingServiceInfo, error) {
		t.Fatal("must not probe an unknown device")
		return 0, nil, nil
	})
	dm := newDeviceManager()
	dm.WarmManualDevicePort("100.65.100.99") // no pairing file for this IP
}
