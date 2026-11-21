package manager

import (
	"errors"
	"reflect"
	"testing"
)

func TestPostInstallRestartsWaitForEachDevice(t *testing.T) {
	var restarted []string
	r := installRestarts{
		devices: make(map[string]*deviceInstalls),
		restart: func(udid string) error {
			restarted = append(restarted, udid)
			return errors.New("device went offline")
		},
	}

	first, second, last := r.begin("living"), r.begin("living"), r.begin("living")
	first(true)
	second(true)
	r.begin("office")(false)
	if len(restarted) != 0 {
		t.Fatalf("unexpected restart while an install was pending: %v", restarted)
	}
	r.begin("bedroom")(true)
	last(false)
	r.begin("living")(false)
	if !reflect.DeepEqual(restarted, []string{"bedroom", "living"}) || len(r.devices) != 0 {
		t.Fatalf("restart attempts = %v, active devices = %d", restarted, len(r.devices))
	}
}

func TestPostInstallAdmissionDuringRestart(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	r := installRestarts{
		devices: make(map[string]*deviceInstalls),
		restart: func(string) error {
			close(started)
			<-release
			return nil
		},
	}
	finish := r.begin("tv")
	go finish(true)
	<-started
	r.begin("other")(false)

	entered := make(chan struct{})
	reserved := make(chan func(bool))
	go func() {
		close(entered)
		reserved <- r.begin("tv")
	}()
	<-entered
	select {
	case <-reserved:
		close(release)
		t.Fatal("accepted an install on the TV while its restart request was in flight")
	default:
	}
	close(release)
	(<-reserved)(false)
}
