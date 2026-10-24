//go:build windows

package signing

import (
	"errors"
	"os"
)

// Workspace locking on Windows: there is no flock(2) and no O_NOFOLLOW.
// LockFileEx (lockfile_windows_syscall.go) takes over for flock: the lock
// files are only ever held, never written. The symlink refusal of the Unix
// side has no direct counterpart; creation still goes through O_EXCL and
// the workspace root itself is refused when it is a symlink, so the staged
// directories under it are not attacker-reachable.

const (
	lockWait = lockExclusive
	lockTry  = lockExclusive | lockFailImmediately
)

// openNoFollow opens path with flags. Windows has no O_NOFOLLOW; see the
// note above on why the staged directories do not need it.
func openNoFollow(path string, flags int) (*os.File, error) {
	return os.OpenFile(path, flags, 0)
}

// flock acquires the whole-file lock how on f.
func flock(f *os.File, how int) error {
	return lockFileRange(f, uint32(how))
}

// lockBusy reports whether err means the lock is held by someone else.
func lockBusy(err error) bool {
	return errors.Is(err, errLockBusy)
}
