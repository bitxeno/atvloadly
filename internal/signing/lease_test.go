package signing

import (
	"testing"
	"time"
)

func TestExclusiveRefusedWhileLeased(t *testing.T) {
	const id, otherID = 101, 102
	release := AcquireLease(id)
	if !LeaseHeld(id) {
		t.Fatal("LeaseHeld = false while a lease is held")
	}

	ran := false
	err := Exclusive(id, func() error { ran = true; return nil })
	if code := testCode(t, err, ClassIdentity); code != CodeIdentityInUse || ran {
		t.Fatalf("Exclusive while leased: code %q, ran %v; want %q and not run", code, ran, CodeIdentityInUse)
	}
	if err := Exclusive(otherID, func() error { return nil }); err != nil {
		t.Fatalf("Exclusive on another identity: %v", err)
	}

	release()
	release()
	if LeaseHeld(id) {
		t.Fatal("LeaseHeld = true after release")
	}
	if err := Exclusive(id, func() error { ran = true; return nil }); err != nil || !ran {
		t.Fatalf("Exclusive after release: %v, ran %v", err, ran)
	}
}

// Releasing the same lease twice must not cancel another task's lease.
func TestLeaseReleaseIsIdempotent(t *testing.T) {
	const id = 103
	first := AcquireLease(id)
	second := AcquireLease(id)
	first()
	first()
	if !LeaseHeld(id) {
		t.Fatal("second lease lost after releasing the first one twice")
	}
	second()
	if LeaseHeld(id) {
		t.Fatal("LeaseHeld = true after releasing every lease")
	}
}

func TestAcquireLeaseWaitsForExclusive(t *testing.T) {
	const id = 104
	started := make(chan struct{})
	finish := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Exclusive(id, func() error {
			close(started)
			<-finish
			return nil
		})
	}()
	<-started

	acquired := make(chan func(), 1)
	go func() { acquired <- AcquireLease(id) }()
	select {
	case release := <-acquired:
		release()
		t.Fatal("lease acquired while an exclusive operation runs")
	case <-time.After(100 * time.Millisecond):
	}

	close(finish)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	select {
	case release := <-acquired:
		if !LeaseHeld(id) {
			t.Fatal("LeaseHeld = false after the lease was acquired")
		}
		release()
	case <-time.After(5 * time.Second):
		t.Fatal("lease not acquired after the exclusive operation ended")
	}
}
