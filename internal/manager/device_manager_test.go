package manager

import (
	"context"
	"testing"
	"time"

	"github.com/bitxeno/atvloadly/internal/model"
)

// devicesCount returns the number of registered devices.
func (dm *DeviceManager) devicesCount() int {
	n := 0
	dm.devices.Range(func(k, v any) bool {
		n++
		return true
	})
	return n
}

func TestCheckPairingThrottle(t *testing.T) {
	dm := newDeviceManager()

	// First check for a device passes.
	if dm.checkPairingThrottle("device-a") {
		t.Errorf("first check should not be throttled")
	}

	// Immediate duplicate is throttled.
	if !dm.checkPairingThrottle("device-a") {
		t.Errorf("duplicate check should be throttled")
	}

	// A different device is not throttled.
	if dm.checkPairingThrottle("device-b") {
		t.Errorf("different device should not be throttled")
	}

	// Backdating the last check lets the device through again.
	dm.pairingMu.Lock()
	dm.pairingCheckedAt["device-a"] = time.Now().Add(-2 * pairingCheckInterval)
	dm.pairingMu.Unlock()

	if dm.checkPairingThrottle("device-a") {
		t.Errorf("check after interval should not be throttled")
	}
}

// TestPairingThrottleClearedOnDisconnect verifies that a disconnect
// (Remove event) clears the throttle, so a reconnect within the throttle
// window is processed immediately instead of being skipped.
func TestPairingThrottleClearedOnDisconnect(t *testing.T) {
	dm := newDeviceManager()

	// Device connects (first check passes) and then re-announces
	// (duplicate is throttled within the window).
	if dm.checkPairingThrottle("device-a") {
		t.Errorf("first check should not be throttled")
	}
	if !dm.checkPairingThrottle("device-a") {
		t.Errorf("duplicate check should be throttled")
	}

	// Simulate the Remove event clearing the throttle for the device.
	dm.clearPairingThrottle("device-a")

	// Reconnect within the window must not be throttled.
	if dm.checkPairingThrottle("device-a") {
		t.Errorf("reconnect after disconnect should not be throttled")
	}
}

// TestUpdateRemotePairingDevice verifies that a throttled Add event still
// refreshes the connection metadata, so a device that reconnects without a
// goodbye reflects its new address instead of keeping stale data.
func TestUpdateRemotePairingDevice(t *testing.T) {
	dm := newDeviceManager()
	dm.SaveDevice(model.Device{
		ID:          "id-1",
		ServiceName: "UUID-1",
		Connection:  model.DeviceConnectionRemote,
		IP:          "192.168.1.100",
		Port:        49152,
	})

	dm.updateRemotePairingDevice("UUID-1", "192.168.1.200", 5000)

	dev, ok := dm.GetDeviceByID("id-1")
	if !ok {
		t.Fatalf("device not found")
	}
	if dev.IP != "192.168.1.200" || dev.Port != 5000 {
		t.Errorf("device metadata not updated: ip=%s port=%d", dev.IP, dev.Port)
	}
}

// TestCancelPairingCheck verifies that cancelling a device's in-flight
// pairing check invokes its cancel func and removes it from the tracking
// map, and that cancelling an unknown device is a no-op.
func TestCancelPairingCheck(t *testing.T) {
	dm := newDeviceManager()

	cancelled := false
	dm.pairingMu.Lock()
	dm.pairingCancel["device-a"] = func() { cancelled = true }
	dm.pairingMu.Unlock()

	dm.cancelPairingCheck("device-a")
	if !cancelled {
		t.Errorf("cancel func should have been called")
	}
	if _, ok := dm.pairingCancel["device-a"]; ok {
		t.Errorf("cancel entry should be removed after cancel")
	}

	// Cancelling unknown devices must not panic.
	dm.cancelPairingCheck("device-a")
	dm.cancelPairingCheck("nonexistent")
}

// TestCheckRemotePairingAsyncCancelled verifies that a cancelled async
// check does not add the device (simulating a Remove arriving mid-check).
func TestCheckRemotePairingAsyncCancelled(t *testing.T) {
	dm := newDeviceManager()

	// Cancelled context: the check must not register the device.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dm.checkRemotePairingAsync(ctx, "UUID-1", "auth", "UUID-1", "iPhone", "192.168.1.100", 49152)

	if n := dm.devicesCount(); n != 0 {
		t.Errorf("no device should be registered after cancelled check, got %d", n)
	}
}
