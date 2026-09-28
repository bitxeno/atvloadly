package signing

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
)

func TestSealerRoundTripAndPersistence(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "keys", "signing-identity.key")
	sealer, err := LoadOrCreateSealer(keyFile)
	if err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]os.FileMode{filepath.Dir(keyFile): 0o700, keyFile: 0o600} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s mode = %04o, want %04o", path, got, want)
		}
	}
	info, _ := os.Stat(keyFile)
	if info.Size() != sealKeySize {
		t.Fatalf("key file size = %d, want %d", info.Size(), sealKeySize)
	}

	secret := []byte("-----BEGIN PRIVATE KEY----- synthetic")
	aad := IdentityAAD("abc123")
	blob, err := sealer.Seal(secret, aad)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, secret) {
		t.Fatal("sealed blob contains the plaintext")
	}

	reloaded, err := LoadOrCreateSealer(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reloaded.Open(blob, aad)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, secret) {
		t.Fatalf("Open = %q, want %q", got, secret)
	}
}

func TestSealerOpenFailures(t *testing.T) {
	dir := t.TempDir()
	sealer, err := LoadOrCreateSealer(filepath.Join(dir, "a.key"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := LoadOrCreateSealer(filepath.Join(dir, "b.key"))
	if err != nil {
		t.Fatal(err)
	}
	aad := IdentityAAD("abc123")
	blob, err := sealer.Seal([]byte("private key"), aad)
	if err != nil {
		t.Fatal(err)
	}
	flip := func(i int) []byte {
		tampered := bytes.Clone(blob)
		tampered[i] ^= 0x01
		return tampered
	}

	tests := []struct {
		name     string
		sealer   *Sealer
		blob     []byte
		aad      []byte
		wantCode string
	}{
		{"different deployment key", other, blob, aad, CodeKeyMismatch},
		{"different identity", sealer, blob, IdentityAAD("def456"), CodeSealedKeyCorrupt},
		{"tampered ciphertext", sealer, flip(sealHeader), aad, CodeSealedKeyCorrupt},
		{"tampered nonce", sealer, flip(sealHeader - 1), aad, CodeSealedKeyCorrupt},
		{"tampered tag", sealer, flip(len(blob) - 1), aad, CodeSealedKeyCorrupt},
		{"bad magic", sealer, flip(0), aad, CodeSealedKeyCorrupt},
		{"truncated", sealer, blob[:sealHeader+sealTagSize-1], aad, CodeSealedKeyCorrupt},
		{"empty", sealer, nil, aad, CodeSealedKeyCorrupt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.sealer.Open(tt.blob, tt.aad)
			if code := testCode(t, err, ClassIdentity); code != tt.wantCode {
				t.Fatalf("code = %q (%v), want %q", code, err, tt.wantCode)
			}
		})
	}
}

func TestLoadOrCreateSealerRejectsBadKeyFile(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name  string
		setup func(path string) error
	}{
		{"too short", func(path string) error { return os.WriteFile(path, make([]byte, sealKeySize-1), 0o600) }},
		{"too long", func(path string) error { return os.WriteFile(path, make([]byte, sealKeySize+1), 0o600) }},
		{"directory", func(path string) error { return os.Mkdir(path, 0o700) }},
		{"unreadable", func(path string) error { return os.WriteFile(path, make([]byte, sealKeySize), 0o000) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "unreadable" && os.Geteuid() == 0 {
				t.Skip("root reads any file")
			}
			path := filepath.Join(dir, tt.name)
			if err := tt.setup(path); err != nil {
				t.Fatal(err)
			}
			_, err := LoadOrCreateSealer(path)
			if code := testCode(t, err, ClassIdentity); code != CodeKeyUnavailable {
				t.Fatalf("code = %q (%v), want %q", code, err, CodeKeyUnavailable)
			}
		})
	}
}

// A shared read-only key file (Docker secrets are 0444) is accepted.
func TestLoadOrCreateSealerAcceptsWorldReadableKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.key")
	if err := os.WriteFile(path, bytes.Repeat([]byte{7}, sealKeySize), 0o444); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateSealer(path); err != nil {
		t.Fatal(err)
	}
}

// Concurrent first uses must all end up with the same key: a creator never
// replaces a key file another one already published.
func TestLoadOrCreateSealerConcurrentCreation(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "keys", "signing-identity.key")
	const n = 16
	sealers := make([]*Sealer, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { sealers[i], errs[i] = LoadOrCreateSealer(keyFile) })
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("sealer %d: %v", i, err)
		}
	}
	aad := IdentityAAD("abc123")
	for i, sealer := range sealers {
		blob, err := sealer.Seal([]byte("private key"), aad)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sealers[(i+1)%n].Open(blob, aad); err != nil {
			t.Fatalf("sealer %d cannot open a blob of sealer %d: %v", (i+1)%n, i, err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(keyFile))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("key directory holds %d entries, want only the key file", len(entries))
	}
}

// withLinkFile replaces the hard link call of createSealKey for one test.
func withLinkFile(t *testing.T, link func(oldname, newname string) error) {
	t.Helper()
	saved := linkFile
	linkFile = link
	t.Cleanup(func() { linkFile = saved })
}

// Filesystems without hard links (vfat, exFAT, some FUSE or SMB mounts) fail
// link(2) with EPERM or EOPNOTSUPP: the key file is created directly instead.
func TestLoadOrCreateSealerWithoutHardLinks(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"EPERM", syscall.EPERM},
		{"EOPNOTSUPP", syscall.EOPNOTSUPP},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withLinkFile(t, func(oldname, newname string) error {
				return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: tt.err}
			})
			keyFile := filepath.Join(t.TempDir(), "keys", "signing-identity.key")
			sealer, err := LoadOrCreateSealer(keyFile)
			if err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(keyFile)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != 0o600 {
				t.Errorf("key file mode = %04o, want 0600", got)
			}
			if info.Size() != sealKeySize {
				t.Errorf("key file size = %d, want %d", info.Size(), sealKeySize)
			}
			entries, err := os.ReadDir(filepath.Dir(keyFile))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Errorf("key directory holds %d entries, want only the key file", len(entries))
			}

			aad := IdentityAAD("abc123")
			blob, err := sealer.Seal([]byte("private key"), aad)
			if err != nil {
				t.Fatal(err)
			}
			reloaded, err := LoadOrCreateSealer(keyFile)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := reloaded.Open(blob, aad); err != nil {
				t.Fatalf("reloaded sealer cannot open a blob of the creating sealer: %v", err)
			}
		})
	}
}

// Without hard links, a key file another creator published first is kept and
// used.
func TestLoadOrCreateSealerWithoutHardLinksKeepsConcurrentKey(t *testing.T) {
	other := bytes.Repeat([]byte{7}, sealKeySize)
	withLinkFile(t, func(oldname, newname string) error {
		// The other creator wins between the link attempt and the fallback.
		if err := os.WriteFile(newname, other, 0o600); err != nil {
			return err
		}
		return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: syscall.EPERM}
	})
	keyFile := filepath.Join(t.TempDir(), "signing-identity.key")
	sealer, err := LoadOrCreateSealer(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(keyFile); err != nil || !bytes.Equal(got, other) {
		t.Fatalf("key file = %x (%v), want the concurrently created key", got, err)
	}
	otherSealer, err := newSealer(other)
	if err != nil {
		t.Fatal(err)
	}
	aad := IdentityAAD("abc123")
	blob, err := sealer.Seal([]byte("private key"), aad)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := otherSealer.Open(blob, aad); err != nil {
		t.Fatalf("sealer does not use the concurrently created key: %v", err)
	}
}

func TestDefaultSealerRetriesAfterFailure(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "signing.key")
	if err := os.WriteFile(keyFile, []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	Configure(keyFile, filepath.Join(dir, "work"))
	t.Cleanup(func() { Configure("", "") })

	if _, err := DefaultSealer(); CodeOf(err) != CodeKeyUnavailable {
		t.Fatalf("DefaultSealer with a bad key file: %v, want %s", err, CodeKeyUnavailable)
	}
	if err := os.Remove(keyFile); err != nil {
		t.Fatal(err)
	}
	if _, err := DefaultSealer(); err != nil {
		t.Fatalf("DefaultSealer after the key file was fixed: %v", err)
	}
}
