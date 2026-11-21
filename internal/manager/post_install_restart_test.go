package manager

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPostInstallRestartsWaitForEachDevice(t *testing.T) {
	restarted := make(chan string)
	release := make(chan struct{})
	r := installRestarts{
		devices: make(map[string]*deviceInstalls),
		restart: func(udid string) error {
			restarted <- udid
			<-release
			return errors.New("device went offline")
		},
	}
	_, first := r.begin("living")
	_, second := r.begin("living")
	_, last := r.begin("living")
	first(true)
	second(true)
	_, other := r.begin("office")
	other(false)
	if ready, finish := r.begin("living"); ready != nil {
		t.Fatal("started a restart before the last install completed")
	} else {
		finish(false)
	}
	last(false)
	if got := <-restarted; got != "living" {
		t.Fatalf("restart for %q", got)
	}

	ready, next := r.begin("living")
	if ready == nil {
		t.Fatal("new work missed the in-flight restart")
	}
	if otherReady, finish := r.begin("office"); otherReady != nil {
		t.Fatal("the other device is blocked")
	} else {
		finish(false)
	}
	_, rejected := r.begin("living")
	rejected(false)
	select {
	case <-ready:
		t.Fatal("allowed device I/O before restart recovery")
	default:
	}
	close(release)
	<-ready
	next(false)
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.devices) != 0 {
		t.Fatal("leaked a device reservation or requested a second restart")
	}
}

func TestWaitForRestartRequiresDisconnectAndRecovery(t *testing.T) {
	observations := []bool{true, true, false, false, true}
	checks := 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := waitForRestart(ctx, time.Millisecond, func() bool {
		checks++
		return observations[checks-1]
	})
	if err != nil || checks != len(observations) {
		t.Fatalf("checks=%d, err=%v", checks, err)
	}
	ctx, cancelTimeout := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancelTimeout()
	if err := waitForRestart(ctx, time.Millisecond, func() bool { return false }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("offline device: %v", err)
	}
}
