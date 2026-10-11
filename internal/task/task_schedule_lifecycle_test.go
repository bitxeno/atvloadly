package task

import (
	"testing"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
)

// Updating settings repeatedly must not wait for the install worker's
// initial usbmuxd readiness delay (30 seconds on Linux).
func TestRunScheduleRepeatedReloadDoesNotBlock(t *testing.T) {
	previous := app.Settings
	settings := &app.SettingsConfiguration{}
	settings.Task.CrodTime = "0,30 3-6 * * *"
	settings.Task.Enabled = true
	settings.Update.CheckInterval = 6
	app.Settings = settings
	defer func() { app.Settings = previous }()

	tk := new()
	defer tk.Stop()
	for i := 0; i < 4; i++ {
		result := make(chan error, 1)
		go func() { result <- tk.RunSchedule() }()
		select {
		case err := <-result:
			if err != nil {
				t.Fatalf("schedule reload %d failed: %v", i+1, err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("schedule reload %d blocked while queue worker was starting", i+1)
		}
	}
}

func TestStopWithoutStartingIsIdempotent(t *testing.T) {
	tk := new()
	tk.Stop()
	tk.Stop()
	select {
	case <-tk.chExitQueue:
	default:
		t.Fatal("queue shutdown channel was not closed")
	}
}
