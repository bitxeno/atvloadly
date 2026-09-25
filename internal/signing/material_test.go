package signing

import (
	"bytes"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/bitxeno/atvloadly/internal/model"
)

// sealedTestIdentity returns a stored identity whose sealed key is key and
// whose certificate is cert.
func sealedTestIdentity(t *testing.T, sealer *Sealer, cert *x509.Certificate, key crypto.Signer) model.SigningIdentity {
	t.Helper()
	sum := sha256.Sum256(cert.Raw)
	sha := hex.EncodeToString(sum[:])
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := sealer.Seal(der, IdentityAAD(sha))
	if err != nil {
		t.Fatal(err)
	}
	return model.SigningIdentity{
		CertificateSHA256: sha,
		CertificateDER:    cert.Raw,
		SealedPrivateKey:  sealed,
		ProfileData:       []byte("synthetic profile bytes"),
	}
}

func TestMaterialize(t *testing.T) {
	configureTestRoot(t)
	sealer, err := LoadOrCreateSealer(filepath.Join(t.TempDir(), "signing.key"))
	if err != nil {
		t.Fatal(err)
	}
	ca := newTestCA(t, "Synthetic WWDR")

	for name, key := range map[string]crypto.Signer{"RSA": testRSAKey(t, 1), "ECDSA": testECKey(t)} {
		t.Run(name, func(t *testing.T) {
			leaf := newTestLeaf(t, key, ca).cert
			identity := sealedTestIdentity(t, sealer, leaf, key)
			ws, err := NewWorkspace()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := ws.Close(); err != nil {
					t.Error(err)
				}
			}()

			material, err := Materialize(ws, identity, sealer)
			if err != nil {
				t.Fatal(err)
			}
			files := map[string][]byte{}
			for _, path := range []string{material.CertificatePath, material.PrivateKeyPath, material.ProfilePath} {
				if filepath.Dir(path) != ws.Dir {
					t.Errorf("%s is not inside the workspace", path)
				}
				info, err := os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
				if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
					t.Errorf("%s: mode %v, want a 0600 regular file", path, info.Mode())
				}
				files[path], _ = os.ReadFile(path)
			}

			certBlocks := pemBlocks(files[material.CertificatePath])
			if len(certBlocks) != 1 || certBlocks[0].Type != "CERTIFICATE" || !bytes.Equal(certBlocks[0].Bytes, leaf.Raw) {
				t.Fatalf("certificate.pem must hold exactly the leaf certificate, got %d blocks", len(certBlocks))
			}
			keyBlocks := pemBlocks(files[material.PrivateKeyPath])
			if len(keyBlocks) != 1 || keyBlocks[0].Type != "PRIVATE KEY" {
				t.Fatalf("private-key.pem must hold one PKCS#8 PRIVATE KEY block, got %d blocks", len(keyBlocks))
			}
			parsed, err := x509.ParsePKCS8PrivateKey(keyBlocks[0].Bytes)
			if err != nil {
				t.Fatal(err)
			}
			if !publicKeysEqual(parsed.(crypto.Signer).Public(), leaf.PublicKey) {
				t.Fatal("materialized key does not match the certificate")
			}
			if !bytes.Equal(files[material.ProfilePath], identity.ProfileData) {
				t.Fatal("profile.mobileprovision differs from the stored profile")
			}
		})
	}
}

func TestMaterializeRejectsKeyNotMatchingCertificate(t *testing.T) {
	configureTestRoot(t)
	sealer, err := LoadOrCreateSealer(filepath.Join(t.TempDir(), "signing.key"))
	if err != nil {
		t.Fatal(err)
	}
	leaf := newTestLeaf(t, testECKey(t), newTestCA(t, "Synthetic WWDR")).cert
	identity := sealedTestIdentity(t, sealer, leaf, testECKey(t))
	ws, err := NewWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ws.Close(); err != nil {
			t.Error(err)
		}
	}()

	_, err = Materialize(ws, identity, sealer)
	if code := testCode(t, err, ClassIdentity); code != CodeSealedKeyCorrupt {
		t.Fatalf("code = %q (%v), want %q", code, err, CodeSealedKeyCorrupt)
	}
	if _, err := os.Lstat(filepath.Join(ws.Dir, "private-key.pem")); err == nil {
		t.Fatal("a private key was written for a mismatching identity")
	}
}

func pemBlocks(data []byte) []*pem.Block {
	var blocks []*pem.Block
	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			return blocks
		}
		blocks = append(blocks, block)
	}
}
