package signing

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// configureTestRoot points the package configuration at a fresh workspace root.
func configureTestRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "work")
	Configure(filepath.Join(dir, "keys", "signing.key"), root)
	t.Cleanup(func() { Configure("", "") })
	return root
}

func TestWorkspaceLifecycle(t *testing.T) {
	root := configureTestRoot(t)
	ws, err := NewWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(ws.Dir) != root {
		t.Fatalf("workspace %s is not under %s", ws.Dir, root)
	}
	for _, dir := range []string{root, ws.Dir, ws.HomeDir, ws.TempDir} {
		info, err := os.Lstat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() || info.Mode().Perm() != 0o700 {
			t.Errorf("%s: mode %v, want a 0700 directory", dir, info.Mode())
		}
	}
	for _, sub := range []string{ws.HomeDir, ws.TempDir} {
		if filepath.Dir(sub) != ws.Dir {
			t.Errorf("%s is not inside the workspace", sub)
		}
	}

	if err := ws.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(ws.Dir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("workspace still exists after Close: %v", err)
	}
	if err := ws.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestNewWorkspaceRefusesSymlinkedRoot(t *testing.T) {
	root := configureTestRoot(t)
	target := t.TempDir()
	if err := os.Symlink(target, root); err != nil {
		t.Fatal(err)
	}
	_, err := NewWorkspace()
	if code := testCode(t, err, ClassSigning); code != CodeWorkspaceFailed {
		t.Fatalf("code = %q (%v), want %q", code, err, CodeWorkspaceFailed)
	}
	if _, err := RecoverWorkspaces(); CodeOf(err) != CodeWorkspaceFailed {
		t.Fatalf("RecoverWorkspaces on a symlinked root: %v, want %s", err, CodeWorkspaceFailed)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("symlink target was written: %d entries", len(entries))
	}
}

func TestRecoverWorkspaces(t *testing.T) {
	root := configureTestRoot(t)

	live, err := NewWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := live.Close(); err != nil {
			t.Error(err)
		}
	}()

	// A crashed task: its lock file exists but nobody holds it anymore.
	crashed, err := NewWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(crashed.Dir, "private-key.pem"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = crashed.state.lock.Close()
	crashed.state.lock = nil

	// A task that crashed before creating its lock file.
	unlocked := filepath.Join(root, "ws-nolock")
	if err := os.Mkdir(unlocked, 0o700); err != nil {
		t.Fatal(err)
	}

	// Entries that are not workspaces are never touched, symlinks never followed.
	outside := t.TempDir()
	keep := filepath.Join(outside, "keep")
	if err := os.WriteFile(keep, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "ws-symlink")); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, "other")
	if err := os.Mkdir(other, 0o700); err != nil {
		t.Fatal(err)
	}

	removed, err := RecoverWorkspaces()
	if err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	for _, path := range []string{crashed.Dir, unlocked} {
		if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("orphan %s still exists: %v", path, err)
		}
	}
	for _, path := range []string{live.Dir, live.HomeDir, keep, other, filepath.Join(root, "ws-symlink")} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("%s was removed: %v", path, err)
		}
	}
}

func TestRecoverWorkspacesWithoutRoot(t *testing.T) {
	configureTestRoot(t)
	removed, err := RecoverWorkspaces()
	if err != nil || removed != 0 {
		t.Fatalf("RecoverWorkspaces = %d, %v; want 0, nil", removed, err)
	}
}
