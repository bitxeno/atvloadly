package signing

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/bitxeno/atvloadly/internal/log"
)

const (
	workspacePrefix = "ws-"
	// lockFileName names both the per-workspace lock and the root lock that
	// serializes workspace creation against recovery.
	lockFileName = ".lock"
)

// Workspace is a private per-task directory holding the materialized signing
// files and the isolated HOME and TMPDIR of the signing engine.
//
// While a workspace is open its lock file is held with flock(2), so a
// RecoverWorkspaces run by any process never removes it.
type Workspace struct {
	Dir     string
	HomeDir string
	TempDir string

	state *workspaceState
}

type workspaceState struct {
	mu   sync.Mutex
	lock *os.File
}

// NewWorkspace creates a workspace under the configured root. Errors are
// *Error of ClassSigning with CodeWorkspaceFailed.
func NewWorkspace() (*Workspace, error) {
	root := workspaceRoot()
	if root == "" {
		return nil, Errorf(ClassSigning, CodeWorkspaceFailed, "the signing workspace root is not configured")
	}
	if err := ensureWorkspaceRoot(root); err != nil {
		return nil, err
	}
	rootLock, err := lockPath(filepath.Join(root, lockFileName), syscall.LOCK_SH)
	if err != nil {
		return nil, Wrap(ClassSigning, CodeWorkspaceFailed, err, "the signing workspace root cannot be locked")
	}
	defer func() { _ = rootLock.Close() }()

	dir, err := os.MkdirTemp(root, workspacePrefix+"*")
	if err != nil {
		return nil, Wrap(ClassSigning, CodeWorkspaceFailed, err, "the signing workspace cannot be created")
	}
	ws, err := openWorkspace(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, Wrap(ClassSigning, CodeWorkspaceFailed, err, "the signing workspace cannot be created")
	}
	return ws, nil
}

// openWorkspace locks the new directory dir and creates its subdirectories.
func openWorkspace(dir string) (*Workspace, error) {
	lock, err := os.OpenFile(filepath.Join(dir, lockFileName), os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close()
		return nil, err
	}
	ws := &Workspace{
		Dir:     dir,
		HomeDir: filepath.Join(dir, "home"),
		TempDir: filepath.Join(dir, "tmp"),
		state:   &workspaceState{lock: lock},
	}
	for _, sub := range []string{ws.HomeDir, ws.TempDir} {
		if err := os.Mkdir(sub, 0o700); err != nil {
			_ = lock.Close()
			return nil, err
		}
	}
	return ws, nil
}

// Close removes the workspace and everything in it, then releases its lock.
// Calling Close again has no effect.
func (w *Workspace) Close() error {
	if w == nil || w.state == nil {
		return nil
	}
	w.state.mu.Lock()
	defer w.state.mu.Unlock()
	if w.state.lock == nil {
		return nil
	}
	err := os.RemoveAll(w.Dir)
	_ = w.state.lock.Close()
	w.state.lock = nil
	if err != nil {
		return Wrap(ClassSigning, CodeWorkspaceFailed, err, "the signing workspace cannot be removed")
	}
	return nil
}

// RecoverWorkspaces removes the workspaces left by crashed processes: every
// ws-* directory of the configured root whose lock is not held. It returns
// the number of removed workspaces.
func RecoverWorkspaces() (int, error) {
	root := workspaceRoot()
	if root == "" {
		return 0, Errorf(ClassSigning, CodeWorkspaceFailed, "the signing workspace root is not configured")
	}
	if _, err := os.Lstat(root); errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err := ensureWorkspaceRoot(root); err != nil {
		return 0, err
	}
	rootLock, err := lockPath(filepath.Join(root, lockFileName), syscall.LOCK_EX)
	if err != nil {
		return 0, Wrap(ClassSigning, CodeWorkspaceFailed, err, "the signing workspace root cannot be locked")
	}
	defer func() { _ = rootLock.Close() }()

	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, Wrap(ClassSigning, CodeWorkspaceFailed, err, "the signing workspace root cannot be read")
	}
	removed := 0
	for _, entry := range entries {
		// DirEntry.Type comes from lstat: symlinks are never followed.
		if !strings.HasPrefix(entry.Name(), workspacePrefix) || !entry.Type().IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		ok, err := removeOrphanWorkspace(dir)
		if err != nil {
			log.Warnf("Could not recover signing workspace %s: %v", dir, err)
			continue
		}
		if ok {
			removed++
		}
	}
	return removed, nil
}

// removeOrphanWorkspace removes dir when it has no lock file or when its lock
// can be acquired. It reports whether dir was removed.
func removeOrphanWorkspace(dir string) (bool, error) {
	lock, err := os.OpenFile(filepath.Join(dir, lockFileName), os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return true, os.RemoveAll(dir)
	}
	if err != nil {
		return false, err
	}
	defer func() { _ = lock.Close() }()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return false, nil
		}
		return false, err
	}
	return true, os.RemoveAll(dir)
}

// ensureWorkspaceRoot creates root (0700) when missing and refuses a root
// that is a symlink or not a directory.
func ensureWorkspaceRoot(root string) error {
	info, err := os.Lstat(root)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(root, 0o700); err != nil {
			return Wrap(ClassSigning, CodeWorkspaceFailed, err, "the signing workspace root cannot be created")
		}
		info, err = os.Lstat(root)
	}
	if err != nil {
		return Wrap(ClassSigning, CodeWorkspaceFailed, err, "the signing workspace root cannot be read")
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return Errorf(ClassSigning, CodeWorkspaceFailed, "the signing workspace root %s is a symlink", root)
	}
	if !info.IsDir() {
		return Errorf(ClassSigning, CodeWorkspaceFailed, "the signing workspace root %s is not a directory", root)
	}
	return nil
}

// lockPath opens (creating it 0600 when missing) and flocks path with how,
// waiting for conflicting holders.
func lockPath(path string, how int) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}
