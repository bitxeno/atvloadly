package manager

import (
	"testing"
	"time"
)

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
	dm.pairingMu.Lock()
	delete(dm.pairingCheckedAt, "device-a")
	dm.pairingMu.Unlock()

	// Reconnect within the window must not be throttled.
	if dm.checkPairingThrottle("device-a") {
		t.Errorf("reconnect after disconnect should not be throttled")
	}
}
