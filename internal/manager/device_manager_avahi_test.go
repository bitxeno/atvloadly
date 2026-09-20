//go:build linux

package manager

import (
	"testing"
)

// TestAvahiBrowsersCloseFreesBrowsersOnce covers the crash reported as
// "panic: close of closed channel" in (*Server).Close during a service scan.
//
// Server.Close frees every signal emitter it still knows about. Freeing a
// browser afterwards closes its already-closed closeCh and panics. Browsers
// are created from per-service-type goroutines while the scan loop may
// return at any moment, so a browser created concurrently with teardown
// used to be freed twice: once by Server.Close and once by its own deferred
// Free.
func TestAvahiBrowsersCloseFreesBrowsersOnce(t *testing.T) {
	browsers := newAvahiBrowsers(nil)

	// Simulate browsers created by the scan loop and by the per-type
	// goroutines.
	freed := make([]string, 0, 2)
	if !browsers.track(func() { freed = append(freed, "typeBrowser") }) {
		t.Fatal("first browser should be tracked")
	}
	if !browsers.track(func() { freed = append(freed, "serviceBrowser") }) {
		t.Fatal("second browser should be tracked")
	}

	browsers.Close()

	if len(freed) != 2 {
		t.Fatalf("every browser should be freed exactly once, got %v", freed)
	}
	if freed[0] != "typeBrowser" || freed[1] != "serviceBrowser" {
		t.Errorf("browsers freed out of order: %v", freed)
	}

	// Close must be idempotent: the deferred Close in ScanServices runs
	// even when the loop already ended.
	browsers.Close()
	if len(freed) != 2 {
		t.Errorf("second Close freed browsers again: %v", freed)
	}
}

// TestAvahiBrowsersRejectsBrowserAfterClose covers a browser created by a
// per-type goroutine after the set was torn down. It must not be freed
// (Server.Close already freed it), otherwise the free panics with "close of
// closed channel".
func TestAvahiBrowsersRejectsBrowserAfterClose(t *testing.T) {
	browsers := newAvahiBrowsers(nil)
	browsers.Close()

	if browsers.track(func() { t.Error("must not free a browser after Close") }) {
		t.Fatal("browser created after Close should be rejected")
	}
}
