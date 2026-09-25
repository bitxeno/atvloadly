package signing

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bitxeno/atvloadly/internal/log"
)

// Sealed blob layout: sealMagic | key id | nonce | AES-256-GCM ciphertext and tag.
const (
	sealMagic     = "ASK1"
	sealKeySize   = 32
	sealKeyIDSize = 8
	sealNonceSize = 12
	sealTagSize   = 16
	sealHeader    = len(sealMagic) + sealKeyIDSize + sealNonceSize
)

// linkFile publishes a key file; tests replace it to simulate filesystems
// without hard links.
var linkFile = os.Link

// Sealer encrypts private keys with AES-256-GCM under the deployment key.
type Sealer struct {
	aead  cipher.AEAD
	keyID [sealKeyIDSize]byte
}

// LoadOrCreateSealer loads the deployment key file, creating it when missing.
// Errors are *Error of ClassIdentity with CodeKeyUnavailable.
func LoadOrCreateSealer(path string) (*Sealer, error) {
	key, err := readSealKey(path)
	if errors.Is(err, fs.ErrNotExist) {
		key, err = createSealKey(path)
	}
	if err != nil {
		return nil, err
	}
	defer clear(key)
	return newSealer(key)
}

func newSealer(key []byte) (*Sealer, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, Wrap(ClassIdentity, CodeKeyUnavailable, err, "the signing key cannot be used")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, Wrap(ClassIdentity, CodeKeyUnavailable, err, "the signing key cannot be used")
	}
	sealer := &Sealer{aead: aead}
	sum := sha256.Sum256(key)
	copy(sealer.keyID[:], sum[:sealKeyIDSize])
	return sealer, nil
}

// readSealKey reads an existing key file. It returns an error matching
// fs.ErrNotExist when the file does not exist.
func readSealKey(path string) ([]byte, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if err != nil {
		return nil, Wrap(ClassIdentity, CodeKeyUnavailable, err, "the signing key file cannot be opened")
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, Wrap(ClassIdentity, CodeKeyUnavailable, err, "the signing key file cannot be read")
	}
	if !info.Mode().IsRegular() || info.Size() != sealKeySize {
		return nil, Errorf(ClassIdentity, CodeKeyUnavailable, "the signing key file %s must be a regular file of exactly %d bytes", path, sealKeySize)
	}
	if info.Mode().Perm()&0o077 != 0 {
		log.Warnf("Signing key file %s is accessible by group or others (mode %04o)", path, info.Mode().Perm())
	}
	key := make([]byte, sealKeySize)
	if _, err := io.ReadFull(f, key); err != nil {
		return nil, Wrap(ClassIdentity, CodeKeyUnavailable, err, "the signing key file cannot be read")
	}
	return key, nil
}

// createSealKey writes a new random key file without ever replacing a key
// created concurrently: the key is written to a temporary file of the same
// directory and hard linked to path, which fails when path already exists and
// never exposes a partial file. On filesystems without hard links (vfat,
// exFAT, some FUSE or SMB mounts) path is created exclusively instead.
func createSealKey(path string) ([]byte, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, Wrap(ClassIdentity, CodeKeyUnavailable, err, "the signing key directory cannot be created")
	}
	key := make([]byte, sealKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, Wrap(ClassIdentity, CodeKeyUnavailable, err, "the signing key cannot be generated")
	}
	tmp, err := os.CreateTemp(dir, ".signing-key-*")
	if err != nil {
		return nil, Wrap(ClassIdentity, CodeKeyUnavailable, err, "the signing key file cannot be created")
	}
	// The temporary name must not keep a copy of the key: after a successful
	// link it is a second name of the published key file.
	defer func() {
		if err := os.Remove(tmp.Name()); err != nil && !errors.Is(err, fs.ErrNotExist) {
			log.Warnf("Could not remove temporary signing key file %s: %v", tmp.Name(), err)
		}
	}()
	_, err = tmp.Write(key)
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, Wrap(ClassIdentity, CodeKeyUnavailable, err, "the signing key file cannot be written")
	}
	err = linkFile(tmp.Name(), path)
	if errors.Is(err, errors.ErrUnsupported) || errors.Is(err, fs.ErrPermission) {
		log.Debugf("Hard link of the signing key file failed (%v), creating %s exclusively", err, path)
		err = writeSealKeyExclusive(path, key)
	}
	if err != nil {
		clear(key)
		if errors.Is(err, fs.ErrExist) {
			// Another process published its key first: use that one.
			key, err := readSealKey(path)
			if errors.Is(err, fs.ErrNotExist) {
				return nil, Wrap(ClassIdentity, CodeKeyUnavailable, err, "the signing key file disappeared while being created")
			}
			return key, err
		}
		return nil, Wrap(ClassIdentity, CodeKeyUnavailable, err, "the signing key file cannot be created")
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	log.Infof("Created signing key file %s", path)
	return key, nil
}

// writeSealKeyExclusive creates path holding key and fails with an error
// matching fs.ErrExist when path already exists. A reader that opens path
// before the key is fully written rejects it by its size. A partially written
// file is removed.
func writeSealKeyExclusive(path string, key []byte) error {
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
			log.Warnf("Could not remove partial signing key file %s: %v", path, removeErr)
		}
		return err
	}
	return nil
}

// Seal encrypts plaintext bound to aad.
func (s *Sealer) Seal(plaintext, aad []byte) ([]byte, error) {
	var nonce [sealNonceSize]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, Wrap(ClassIdentity, CodeKeyUnavailable, err, "a sealing nonce cannot be generated")
	}
	blob := make([]byte, 0, sealHeader+len(plaintext)+sealTagSize)
	blob = append(blob, sealMagic...)
	blob = append(blob, s.keyID[:]...)
	blob = append(blob, nonce[:]...)
	return s.aead.Seal(blob, nonce[:], plaintext, aad), nil
}

// Open decrypts a blob produced by Seal with the same aad.
func (s *Sealer) Open(blob, aad []byte) ([]byte, error) {
	if len(blob) < sealHeader+sealTagSize || string(blob[:len(sealMagic)]) != sealMagic {
		return nil, Errorf(ClassIdentity, CodeSealedKeyCorrupt, "the sealed private key is corrupt")
	}
	keyID := blob[len(sealMagic) : len(sealMagic)+sealKeyIDSize]
	if !bytes.Equal(keyID, s.keyID[:]) {
		return nil, Errorf(ClassIdentity, CodeKeyMismatch,
			"the private key was sealed with a different deployment key; restore the original key file or re-import the identity")
	}
	nonce := blob[len(sealMagic)+sealKeyIDSize : sealHeader]
	plaintext, err := s.aead.Open(nil, nonce, blob[sealHeader:], aad)
	if err != nil {
		return nil, Errorf(ClassIdentity, CodeSealedKeyCorrupt, "the sealed private key is corrupt")
	}
	return plaintext, nil
}

// IdentityAAD returns the additional authenticated data binding a sealed key
// to the identity certificate.
func IdentityAAD(certificateSHA256 string) []byte {
	return []byte("atvloadly-signing-identity:v1:" + certificateSHA256)
}
