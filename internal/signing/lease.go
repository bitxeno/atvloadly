package signing

import "sync"

// In-process coordination between running tasks (leases) and operations that
// modify or delete an identity (exclusive sections).
var (
	leaseMu        sync.Mutex
	leaseCond      = sync.NewCond(&leaseMu)
	leaseCount     = map[uint]int{}
	leaseExclusive = map[uint]bool{}
)

// AcquireLease marks the identity as used by a running task until release is
// called. It waits while an Exclusive operation runs on the identity. Calling
// release more than once has no effect.
func AcquireLease(identityID uint) (release func()) {
	leaseMu.Lock()
	for leaseExclusive[identityID] {
		leaseCond.Wait()
	}
	leaseCount[identityID]++
	leaseMu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			leaseMu.Lock()
			defer leaseMu.Unlock()
			if leaseCount[identityID]--; leaseCount[identityID] <= 0 {
				delete(leaseCount, identityID)
			}
		})
	}
}

// Exclusive runs fn only when no lease is held on the identity and blocks new
// leases while fn runs. It returns an *Error with CodeIdentityInUse otherwise.
// Concurrent Exclusive calls on the same identity run one after the other.
func Exclusive(identityID uint, fn func() error) error {
	leaseMu.Lock()
	for leaseExclusive[identityID] {
		leaseCond.Wait()
	}
	if leaseCount[identityID] > 0 {
		leaseMu.Unlock()
		return Errorf(ClassIdentity, CodeIdentityInUse, "the signing identity is used by a running installation")
	}
	leaseExclusive[identityID] = true
	leaseMu.Unlock()

	defer func() {
		leaseMu.Lock()
		delete(leaseExclusive, identityID)
		leaseMu.Unlock()
		leaseCond.Broadcast()
	}()
	return fn()
}

// LeaseHeld reports whether a running task holds a lease on the identity.
func LeaseHeld(identityID uint) bool {
	leaseMu.Lock()
	defer leaseMu.Unlock()
	return leaseCount[identityID] > 0
}
