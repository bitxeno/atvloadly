//go:build windows

package signing

import (
	"os"
	"syscall"
	"unsafe"
)

// LockFileEx plumbing for lockfile_windows.go: kernel32's LockFileEx is not
// in the syscall package, so it is loaded lazily.

const (
	lockExclusive       = 0x2
	lockFailImmediately = 0x1

	errLockBusy syscall.Errno = 33 // ERROR_LOCK_VIOLATION
)

// lockFileRange locks the first 4 GiB of the file exclusively; the lock
// files are only ever held, never written.
func lockFileRange(f *os.File, flags uint32) error {
	var ol syscall.Overlapped
	r1, _, e1 := lockFileEx.Call(uintptr(f.Fd()),
		uintptr(flags), 0,
		uintptr(^uint32(0)), 0,
		uintptr(unsafe.Pointer(&ol)))
	if r1 == 0 {
		return e1
	}
	return nil
}

var lockFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")
