package signing

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"syscall"

	"github.com/bitxeno/atvloadly/internal/model"
)

// Material lists the signing files written into a workspace.
type Material struct {
	CertificatePath string
	PrivateKeyPath  string
	ProfilePath     string
}

// Materialize unseals the identity private key and writes the leaf
// certificate PEM, the unencrypted PKCS#8 key PEM and the profile into ws.
// The certificate file holds the leaf only: the signing engine keeps the last
// CERTIFICATE block it reads.
func Materialize(ws *Workspace, identity model.SigningIdentity, sealer *Sealer) (*Material, error) {
	if ws == nil || ws.Dir == "" {
		return nil, Errorf(ClassSigning, CodeWorkspaceFailed, "no signing workspace")
	}
	if sealer == nil {
		return nil, Errorf(ClassIdentity, CodeKeyUnavailable, "the signing key is not available")
	}
	if len(identity.ProfileData) == 0 {
		return nil, Errorf(ClassIdentity, CodeProfileInvalid, "the identity has no provisioning profile")
	}
	cert, err := x509.ParseCertificate(identity.CertificateDER)
	if err != nil {
		return nil, Wrap(ClassIdentity, CodeCertificateInvalid, err, "the stored certificate of the identity cannot be parsed")
	}
	keyDER, err := sealer.Open(identity.SealedPrivateKey, IdentityAAD(identity.CertificateSHA256))
	if err != nil {
		return nil, err
	}
	defer clear(keyDER)
	key, err := x509.ParsePKCS8PrivateKey(keyDER)
	if err != nil {
		return nil, Wrap(ClassIdentity, CodeSealedKeyCorrupt, err, "the unsealed private key cannot be parsed")
	}
	signer, ok := key.(crypto.Signer)
	if !ok || !publicKeysEqual(signer.Public(), cert.PublicKey) {
		return nil, Errorf(ClassIdentity, CodeSealedKeyCorrupt, "the unsealed private key does not match the certificate")
	}

	material := &Material{
		CertificatePath: filepath.Join(ws.Dir, "certificate.pem"),
		PrivateKeyPath:  filepath.Join(ws.Dir, "private-key.pem"),
		ProfilePath:     filepath.Join(ws.Dir, "profile.mobileprovision"),
	}
	if err := writeSecretFile(material.CertificatePath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})); err != nil {
		return nil, err
	}
	// Presized so the buffer never reallocates and leaves key copies behind.
	var keyPEM bytes.Buffer
	keyPEM.Grow(2*len(keyDER) + 128)
	err = pem.Encode(&keyPEM, &pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err == nil {
		err = writeSecretFile(material.PrivateKeyPath, keyPEM.Bytes())
	}
	clear(keyPEM.Bytes())
	if err != nil {
		return nil, err
	}
	if err := writeSecretFile(material.ProfilePath, identity.ProfileData); err != nil {
		return nil, err
	}
	return material, nil
}

// writeSecretFile creates path (0600, never replacing an existing file or
// following a symlink) with data.
func writeSecretFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return Wrap(ClassSigning, CodeWorkspaceFailed, err, "the signing material cannot be written")
	}
	_, err = f.Write(data)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return Wrap(ClassSigning, CodeWorkspaceFailed, err, "the signing material cannot be written")
	}
	return nil
}
