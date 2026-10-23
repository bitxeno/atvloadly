//go:build !linux

package manager

import (
	"context"
	"net/netip"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/betamos/zeroconf"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/utils"
)

func TestMDNSAddressUpdateDiscoversDevice(t *testing.T) {
	dm := newDeviceManager()
	dm.handleMDNSEvent(context.Background(), zeroconf.Event{})
	svc := &zeroconf.Service{
		Type: zeroconf.NewType(mdnsServiceRemoteManualPairing),
		Name: "Living Room", Hostname: "Apple-TV.local", Port: 49152,
		Text: []string{"identifier=apple-tv", "name=Living Room"},
	}
	dm.handleMDNSEvent(context.Background(), zeroconf.Event{Service: svc, Op: zeroconf.OpAdded})
	if len(dm.GetDevices()) != 0 {
		t.Fatal("service without a usable address must wait for an update")
	}
	svc.Addrs = []netip.Addr{netip.MustParseAddr("192.168.1.12")}
	dm.handleMDNSEvent(context.Background(), zeroconf.Event{Service: svc, Op: zeroconf.OpUpdated})
	dev, ok := dm.GetDeviceByID(utils.Md5(svc.Name))
	if !ok || dev.IP != "192.168.1.12" || dev.Port != svc.Port {
		t.Fatalf("updated service not discovered: %v", dev)
	}
}

func TestMDNSRemoteUpdateAndGoodbye(t *testing.T) {
	dm := newDeviceManager()
	const name, identifier = "Living Room", "pairing-id"
	dm.SaveDevice(model.Device{
		ID: "device", UDID: "udid", ServiceName: name,
		Connection: model.DeviceConnectionRemote, IP: "192.168.1.10",
	})
	dm.checkPairingThrottle(identifier)
	checkCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dm.pairingCancel[identifier] = &pairingCheck{cancel: cancel}
	svc := &zeroconf.Service{
		Type: zeroconf.NewType(mdnsServiceRemotePairing), Name: name,
		Addrs: []netip.Addr{netip.MustParseAddr("192.168.1.20")}, Port: 50000,
		Text: []string{"identifier=" + identifier, "authTag=test-auth-tag"},
	}
	dm.handleMDNSEvent(context.Background(), zeroconf.Event{Service: svc, Op: zeroconf.OpUpdated})
	dev, ok := dm.GetDeviceByID("device")
	if !ok || dev.IP != "192.168.1.20" || dev.Port != 50000 {
		t.Fatalf("paired device retained stale connection: %v", dev)
	}
	dm.handleMDNSEvent(context.Background(), zeroconf.Event{Service: svc, Op: zeroconf.OpRemoved})
	if checkCtx.Err() == nil {
		t.Error("goodbye did not cancel the check keyed by identifier")
	}
	if dm.checkPairingThrottle(identifier) {
		t.Error("goodbye did not clear the identifier throttle")
	}
	if len(dm.GetDevices()) != 0 {
		t.Error("goodbye left a disconnected device visible")
	}
}

func mobdev2Event(op zeroconf.Op) zeroconf.Event {
	return zeroconf.Event{Op: op, Service: &zeroconf.Service{
		Type: zeroconf.NewType(mdnsServiceAppleMobdev2), Name: "aa:bb:cc:dd:ee:ff@fe80::1",
		Hostname: "Apple-TV.local", Addrs: []netip.Addr{netip.MustParseAddr("192.168.1.30")}, Port: 32498,
	}}
}

func shortLockdownLookupDelays(t *testing.T, delays ...time.Duration) {
	previous := lockdownLookupDelays
	lockdownLookupDelays = delays
	t.Cleanup(func() { lockdownLookupDelays = previous })
}

// macOS usbmuxd attaches a Wi-Fi device after seeing the same announcement,
// so the first pair record lookup can miss a device that is paired.
func TestMDNSLockdownWaitsForUsbmuxdToAttachDevice(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("pair records are read from usbmuxd only on macOS")
	}
	var attached atomic.Bool
	fakeUsbmuxd(t, func() []string {
		if attached.Swap(true) {
			return []string{"udid"}
		}
		return nil
	}, map[string]map[string]any{"udid": {"WiFiMACAddress": "AA:BB:CC:DD:EE:FF"}})
	shortLockdownLookupDelays(t, 0, 10*time.Millisecond, 10*time.Millisecond)

	dm := newDeviceManager()
	connected := make(chan model.Device, 1)
	dm.onDeviceConnected = func(device model.Device) { connected <- device }
	dm.handleMDNSEvent(context.Background(), mobdev2Event(zeroconf.OpAdded))
	select {
	case dev := <-connected:
		if dev.UDID != "udid" || dev.Connection != model.DeviceConnectionLockdown || dev.Status != model.Paired ||
			dev.IP != "192.168.1.30" || dev.DeviceClass == "" {
			t.Fatalf("unexpected lockdown device: %+v", dev)
		}
	case <-time.After(time.Second):
		t.Fatal("the device was not connected once usbmuxd attached it")
	}
	if dev, ok := dm.GetDeviceByID(utils.Md5("udid")); !ok || dev.UDID != "udid" {
		t.Fatalf("lockdown device not saved by its ID: %v", dev)
	}
}

func TestMDNSLockdownGoodbyeCancelsLookup(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("pair records are read from usbmuxd only on macOS")
	}
	var attached atomic.Bool
	fakeUsbmuxd(t, func() []string {
		if attached.Load() {
			return []string{"udid"}
		}
		return nil
	}, map[string]map[string]any{"udid": {"WiFiMACAddress": "aa:bb:cc:dd:ee:ff"}})
	shortLockdownLookupDelays(t, 0, 100*time.Millisecond)

	dm := newDeviceManager()
	dm.handleMDNSEvent(context.Background(), mobdev2Event(zeroconf.OpAdded))
	time.Sleep(30 * time.Millisecond)
	dm.handleMDNSEvent(context.Background(), mobdev2Event(zeroconf.OpRemoved))
	attached.Store(true)
	time.Sleep(200 * time.Millisecond)
	if got := len(dm.GetDevices()); got != 0 {
		t.Fatalf("devices = %d, a device that said goodbye was re-added", got)
	}
	// Taking pairingMu also orders the finished lookup before the cleanup
	// that restores the usbmuxd socket path.
	dm.pairingMu.Lock()
	defer dm.pairingMu.Unlock()
	if n := len(dm.lockdownLookups); n != 0 {
		t.Fatalf("%d lockdown lookups still registered after goodbye", n)
	}
}

// macOS keeps the lockdown record after remote pairing succeeds, so a later
// mobdev2 announcement must not replace the remote connection.
func TestMDNSLockdownKeepsRemoteConnection(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("pair records are read from usbmuxd only on macOS")
	}
	fakeUsbmuxd(t, func() []string { return []string{"udid"} },
		map[string]map[string]any{"udid": {"WiFiMACAddress": "aa:bb:cc:dd:ee:ff"}})
	shortLockdownLookupDelays(t, 0)

	dm := newDeviceManager()
	remote := model.Device{ID: utils.Md5("udid"), UDID: "udid", Connection: model.DeviceConnectionRemote, Status: model.Paired}
	dm.SaveDevice(remote)
	connected := make(chan model.Device, 1)
	dm.onDeviceConnected = func(device model.Device) { connected <- device }
	dm.handleMDNSEvent(context.Background(), mobdev2Event(zeroconf.OpUpdated))

	// The lookup deregisters itself once it has finished.
	deadline := time.Now().Add(time.Second)
	for {
		dm.pairingMu.Lock()
		pending := len(dm.lockdownLookups)
		dm.pairingMu.Unlock()
		if pending == 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	select {
	case dev := <-connected:
		t.Fatalf("lockdown lookup reconnected the remote device: %+v", dev)
	default:
	}
	if dev, ok := dm.GetDeviceByUDID("udid"); !ok || dev.Connection != model.DeviceConnectionRemote {
		t.Fatalf("remote device replaced by lockdown: %+v", dev)
	}
}
