//go:build linux

package manager

import (
	"testing"
)

// TestAvahiScanSessionCloseFreesBrowsersOnce covers the crash reported as
// "panic: close of closed channel" in (*Server).Close during a service scan.
//
// Server.Close frees every signal emitter it still knows about. Freeing a
// browser afterwards closes its already-closed closeCh and panics. Browsers
// are created from per-service-type goroutines while the scan loop may
// return at any moment, so a browser created concurrently with teardown
// used to be freed twice: once by Server.Close and once by its own deferred
// Free.
func TestAvahiScanSessionCloseFreesBrowsersOnce(t *testing.T) {
	session := newAvahiScanSession(nil)

	// Simulate browsers created by the scan loop and by the per-type
	// goroutines.
	freed := make([]string, 0, 2)
	if !session.track(func() { freed = append(freed, "typeBrowser") }) {
		t.Fatal("first browser should be tracked")
	}
	if !session.track(func() { freed = append(freed, "serviceBrowser") }) {
		t.Fatal("second browser should be tracked")
	}

	session.close()

	if len(freed) != 2 {
		t.Fatalf("every browser should be freed exactly once, got %v", freed)
	}
	if freed[0] != "typeBrowser" || freed[1] != "serviceBrowser" {
		t.Errorf("browsers freed out of order: %v", freed)
	}

	// Close must be idempotent: the deferred close in ScanServices runs
	// even when the loop already ended.
	session.close()
	if len(freed) != 2 {
		t.Errorf("second close freed browsers again: %v", freed)
	}
}

// TestAvahiScanSessionRejectsBrowserAfterClose covers a browser created by a
// per-type goroutine after the session was torn down. It must not be freed
// (Server.Close already freed it), otherwise the free panics with "close of
// closed channel".
func TestAvahiScanSessionRejectsBrowserAfterClose(t *testing.T) {
	session := newAvahiScanSession(nil)
	session.close()

	if session.track(func() { t.Error("must not free a browser after close") }) {
		t.Fatal("browser created after close should be rejected")
	}
}
