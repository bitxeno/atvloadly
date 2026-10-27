package service

import (
	"testing"

	"github.com/bitxeno/atvloadly/internal/app"
)

// TestGetServiceStatusEmptyWhenZeroconfDisabled verifies that disabling
// zeroconf reports no discovery service, which the home page uses to hide the
// service-status module.
func TestGetServiceStatusEmptyWhenZeroconfDisabled(t *testing.T) {
	orig := app.Config
	t.Cleanup(func() { app.Config = orig })

	app.Config = &app.Configuration{}
	app.Config.Zeroconf = false

	if got := GetServiceStatus(); len(got) != 0 {
		t.Fatalf("disabled zeroconf should report no services, got %v", got)
	}
}
