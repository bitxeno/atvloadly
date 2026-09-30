package secret

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bitxeno/atvloadly/internal/log"
)

// Sealed blob layout: secretMagic | key id | nonce | AES-256-GCM ciphertext and tag.
const (
	secretMagic   = "SEC1"
	secretKeySize = 32
	secretKeyID   = 8
	secretNonce   = 12
	secretTag     = 16
	secretHeader  = len(secretMagic) + secretKeyID + secretNonce
)

// Sentinel errors returned by this package.
var (
	ErrUnavailable = errors.New("the secret key is unavailable")
	ErrCorrupt     = errors.New("the sealed secret is corrupt")
	ErrKeyMismatch = errors.New("the secret was sealed with a different key file")
)

// Store encrypts small secrets with AES-256-GCM under a dedicated key file.
type Store struct {
	aead  cipher.AEAD
	keyID [secretKeyID]byte
}

// LoadOrCreate loads the key file, creating it when missing. The keys
// directory is created as needed, so a missing keys directory is not an error.
func LoadOrCreate(path string) (*Store, error) {
	key, err := readSecretKey(path)
	if errors.Is(err, fs.ErrNotExist) {
		key, err = createSecretKey(path)
	}
	if err != nil {
		return nil, err
	}
	defer clear(key)
	return newStore(key)
}

func newStore(key []byte) (*Store, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	s := &Store{aead: aead}
	sum := sha256.Sum256(key)
	copy(s.keyID[:], sum[:secretKeyID])
	return s, nil
}

func readSecretKey(path string) ([]byte, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("%w: cannot open %s: %v", ErrUnavailable, path, err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("%w: cannot read %s: %v", ErrUnavailable, path, err)
	}
	if !info.Mode().IsRegular() || info.Size() != secretKeySize {
		return nil, fmt.Errorf("%w: %s must be a regular file of exactly %d bytes", ErrUnavailable, path, secretKeySize)
	}
	if info.Mode().Perm()&0o077 != 0 {
		log.Warnf("Secret key file %s is accessible by group or others (mode %04o)", path, info.Mode().Perm())
	}
	key := make([]byte, secretKeySize)
	if _, err := io.ReadFull(f, key); err != nil {
		return nil, fmt.Errorf("%w: cannot read %s: %v", ErrUnavailable, path, err)
	}
	return key, nil
}

// createSecretKey writes a new random key file without ever replacing a key
// created concurrently: the key is written to a temporary file in the same
// directory and hard linked to path, which fails when path already exists and
// never exposes a partial file. On filesystems without hard links path is
// created exclusively instead.
func createSecretKey(path string) ([]byte, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("%w: cannot create %s: %v", ErrUnavailable, dir, err)
	}
	key := make([]byte, secretKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("%w: cannot generate: %v", ErrUnavailable, err)
	}
	tmp, err := os.CreateTemp(dir, ".secret-key-*")
	if err != nil {
		return nil, fmt.Errorf("%w: cannot create: %v", ErrUnavailable, err)
	}
	// The temporary name must not keep a copy of the key: after a successful
	// link it is a second name of the published key file.
	defer func() {
		if err := os.Remove(tmp.Name()); err != nil && !errors.Is(err, fs.ErrNotExist) {
			log.Warnf("Could not remove temporary secret key file %s: %v", tmp.Name(), err)
		}
	}()
	if _, err = tmp.Write(key); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, fmt.Errorf("%w: cannot write: %v", ErrUnavailable, err)
	}
	err = os.Link(tmp.Name(), path)
	if errors.Is(err, errors.ErrUnsupported) || errors.Is(err, fs.ErrPermission) {
		log.Debugf("Hard link of the secret key file failed (%v), creating %s exclusively", err, path)
		err = writeSecretKeyExclusive(path, key)
	}
	if err != nil {
		clear(key)
		if errors.Is(err, fs.ErrExist) {
			// Another process published its key first: use that one.
			key, err := readSecretKey(path)
			if errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("%w: %s disappeared while being created", ErrUnavailable, path)
			}
			return key, err
		}
		return nil, fmt.Errorf("%w: cannot create %s: %v", ErrUnavailable, path, err)
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	log.Infof("Created secret key file %s", path)
	return key, nil
}

// writeSecretKeyExclusive creates path holding key and fails with an error
// matching fs.ErrExist when path already exists.
func writeSecretKeyExclusive(path string, key []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(key)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			log.Warnf("Could not remove partial secret key file %s: %v", path, removeErr)
		}
		return err
	}
	return nil
}

// Seal encrypts plaintext bound to aad.
func (s *Store) Seal(plaintext, aad []byte) ([]byte, error) {
	var nonce [secretNonce]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("%w: cannot seal: %v", ErrUnavailable, err)
	}
	blob := make([]byte, 0, secretHeader+len(plaintext)+secretTag)
	blob = append(blob, secretMagic...)
	blob = append(blob, s.keyID[:]...)
	blob = append(blob, nonce[:]...)
	return s.aead.Seal(blob, nonce[:], plaintext, aad), nil
}

// Open decrypts a blob produced by Seal with the same aad.
func (s *Store) Open(blob, aad []byte) ([]byte, error) {
	if len(blob) < secretHeader+secretTag || string(blob[:len(secretMagic)]) != secretMagic {
		return nil, ErrCorrupt
	}
	if keyID := blob[len(secretMagic) : len(secretMagic)+secretKeyID]; !bytes.Equal(keyID, s.keyID[:]) {
		return nil, ErrKeyMismatch
	}
	nonce := blob[len(secretMagic)+secretKeyID : secretHeader]
	plaintext, err := s.aead.Open(nil, nonce, blob[secretHeader:], aad)
	if err != nil {
		return nil, ErrCorrupt
	}
	return plaintext, nil
}

// SealString seals plaintext and returns the base64 blob for settings storage.
func (s *Store) SealString(plaintext, aad string) (string, error) {
	blob, err := s.Seal([]byte(plaintext), []byte(aad))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(blob), nil
}

// OpenString opens a base64 blob produced by SealString. An empty sealed
// value means not configured and returns "" with no error.
func (s *Store) OpenString(sealed, aad string) (string, error) {
	if sealed == "" {
		return "", nil
	}
	blob, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", ErrCorrupt
	}
	plaintext, err := s.Open(blob, []byte(aad))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
