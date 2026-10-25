//go:build unix

package signing

import (
	"errors"
	"os"
	"syscall"
)

// The Unix side of the workspace locking: flock(2) and O_NOFOLLOW live only
// on Unix, so workspace.go and material.go go through these helpers
// (lockfile_windows.go holds the counterpart).

// Lock constants for lockPath and flock: wait blocks on a conflicting
// holder, try fails instead.
const (
	lockWait = syscall.LOCK_EX
	lockTry  = syscall.LOCK_EX | syscall.LOCK_NB
)

// openNoFollow opens path with flags, refusing to follow a symlink at path.
func openNoFollow(path string, flags int) (*os.File, error) {
	return os.OpenFile(path, flags|syscall.O_NOFOLLOW, 0)
}

// flock acquires the lock how on f.
func flock(f *os.File, how int) error {
	return syscall.Flock(int(f.Fd()), how)
}

// lockBusy reports whether err means the lock is held by someone else.
func lockBusy(err error) bool {
	return errors.Is(err, syscall.EWOULDBLOCK)
}
