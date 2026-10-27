//go:build linux

package manager

import (
	"errors"
	"testing"
	"time"

	"github.com/holoplot/go-avahi"
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

// TestStartSkipsDiscoveryWhenAvahiUnavailable covers issue #235: on a host
// without Avahi/D-Bus (manual-IP / Tailscale deployments) Start must not keep
// retrying the connection. It should give up immediately instead of logging
// "Avahi server unavailable" every few seconds.
func TestStartSkipsDiscoveryWhenAvahiUnavailable(t *testing.T) {
	orig := avahiAvailable
	avahiAvailable = func() (*avahi.Server, error) {
		return nil, errors.New("cannot get system bus: no such file or directory")
	}
	t.Cleanup(func() { avahiAvailable = orig })

	dm := newDeviceManager()

	done := make(chan struct{})
	go func() {
		dm.Start()
		close(done)
	}()

	select {
	case <-done:
		// Start returned without ever running discovery.
	case <-time.After(2 * time.Second):
		dm.Stop()
		t.Fatal("Start did not return when Avahi was unavailable")
	}
}
