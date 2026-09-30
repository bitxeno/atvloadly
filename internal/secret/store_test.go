package secret

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestStoreRoundTripAndPersistence(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "keys", "secret-store.key")
	s, err := LoadOrCreate(keyFile)
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

	plaintext := []byte("ghp_1234567890abcdef")
	aad := []byte("atvloadly-github-token:v1")
	blob, err := s.Seal(plaintext, aad)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, plaintext) {
		t.Fatal("sealed blob contains the plaintext")
	}

	reloaded, err := LoadOrCreate(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reloaded.Open(blob, aad)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("Open = %q, want %q", got, plaintext)
	}
}

func TestStoreStringHelpers(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "secret-store.key")
	s, err := LoadOrCreate(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := s.SealString("token12345678", "aad:v1")
	if err != nil {
		t.Fatal(err)
	}
	if sealed == "" || sealed == "token12345678" {
		t.Fatal("SealString did not seal")
	}
	got, err := s.OpenString(sealed, "aad:v1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "token12345678" {
		t.Fatalf("OpenString = %q", got)
	}
	if got, err := s.OpenString("", "aad:v1"); got != "" || err != nil {
		t.Fatalf("OpenString empty = %q, %v; want empty, nil", got, err)
	}
	if _, err := s.OpenString("!!!", "aad:v1"); err == nil {
		t.Fatal("OpenString accepts invalid base64")
	}
}

func TestStoreOpenFailures(t *testing.T) {
	dir := t.TempDir()
	s, err := LoadOrCreate(filepath.Join(dir, "a.key"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := LoadOrCreate(filepath.Join(dir, "b.key"))
	if err != nil {
		t.Fatal(err)
	}
	aad := []byte("aad:v1")
	blob, err := s.Seal([]byte("secret-value"), aad)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Open(blob, aad); err != ErrKeyMismatch {
		t.Fatalf("different key: err = %v, want ErrKeyMismatch", err)
	}
	if _, err := s.Open(blob, []byte("other:v1")); err != ErrCorrupt {
		t.Fatalf("different aad: err = %v, want ErrCorrupt", err)
	}
	tampered := bytes.Clone(blob)
	tampered[len(tampered)-1] ^= 0x01
	if _, err := s.Open(tampered, aad); err != ErrCorrupt {
		t.Fatalf("tampered: err = %v, want ErrCorrupt", err)
	}
	if _, err := s.Open(blob[:len(blob)-1], aad); err != ErrCorrupt {
		t.Fatalf("truncated: err = %v, want ErrCorrupt", err)
	}
}

func TestLoadOrCreateRejectsBadKeyFile(t *testing.T) {
	dir := t.TempDir()
	for _, size := range []int{secretKeySize - 1, secretKeySize + 1} {
		path := filepath.Join(dir, filepath.Base(t.TempDir())+string(rune(size)))
		if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadOrCreate(path); err == nil {
			t.Fatalf("size %d accepted", size)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "adir"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreate(filepath.Join(dir, "adir")); err == nil {
		t.Fatal("directory accepted as key file")
	}
}

func TestLoadOrCreateConcurrentCreation(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "keys", "secret-store.key")
	const n = 16
	stores := make([]*Store, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { stores[i], errs[i] = LoadOrCreate(keyFile) })
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("store %d: %v", i, err)
		}
	}
	aad := []byte("aad:v1")
	for i, s := range stores {
		blob, err := s.Seal([]byte("secret-value"), aad)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := stores[(i+1)%n].Open(blob, aad); err != nil {
			t.Fatalf("store %d cannot open a blob of store %d: %v", (i+1)%n, i, err)
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

func TestDefaultRetriesAfterFailure(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "secret.key")
	if err := os.WriteFile(keyFile, []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	Configure(keyFile)
	t.Cleanup(func() { Configure("") })

	if _, err := Default(); err == nil {
		t.Fatal("Default with a bad key file succeeds")
	}
	if err := os.Remove(keyFile); err != nil {
		t.Fatal(err)
	}
	if _, err := Default(); err != nil {
		t.Fatalf("Default after the key file was fixed: %v", err)
	}
}
