package signing

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"errors"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// P12Identity is the single code signing identity (leaf certificate matching
// the private key) found in a PKCS#12 file.
type P12Identity struct {
	Certificate *x509.Certificate
	PrivateKey  crypto.Signer
}

// Error texts of go-pkcs12 v0.7.3 that have no exported sentinel.
const (
	pkcs12ErrManyKeys      = "pkcs12: expected exactly one key bag"
	pkcs12ErrNoPrivateKey  = "pkcs12: private key missing"
	pkcs12ErrNoCertificate = "pkcs12: certificate missing"
	pkcs12ErrNoMAC         = "pkcs12: no MAC in data"
)

// DecodeP12 decodes a PKCS#12 file in-process. The password may be empty.
// Errors are *Error of ClassIdentity with a CodeP12* code.
func DecodeP12(data []byte, password string) (*P12Identity, error) {
	if len(data) > MaxP12Size {
		return nil, Errorf(ClassIdentity, CodeUploadTooLarge, "the PKCS#12 file exceeds %d bytes", MaxP12Size)
	}
	if err := checkP12Iterations(data); err != nil {
		return nil, err
	}
	key, leaf, chain, err := pkcs12.DecodeChain(data, password)
	if err != nil {
		return nil, p12Error(err)
	}
	signer, err := p12Signer(key)
	if err != nil {
		return nil, err
	}
	cert, err := p12IdentityCertificate(signer, append([]*x509.Certificate{leaf}, chain...))
	if err != nil {
		return nil, err
	}
	return &P12Identity{Certificate: cert, PrivateKey: signer}, nil
}

// p12Error maps a go-pkcs12 decoding error to a stable code. The library
// never includes the password in its errors.
func p12Error(err error) *Error {
	var notImplemented pkcs12.NotImplementedError
	switch {
	case errors.Is(err, pkcs12.ErrIncorrectPassword):
		return Errorf(ClassIdentity, CodeP12WrongPassword, "the PKCS#12 password is incorrect")
	case errors.As(err, &notImplemented):
		return Wrap(ClassIdentity, CodeP12Unsupported, err, "the PKCS#12 file uses an unsupported format")
	}
	switch err.Error() {
	case pkcs12ErrNoMAC:
		return Errorf(ClassIdentity, CodeP12WrongPassword, "the PKCS#12 file is not password protected: leave the password empty")
	case pkcs12ErrManyKeys:
		return Errorf(ClassIdentity, CodeP12Ambiguous, "the PKCS#12 file contains more than one private key")
	case pkcs12ErrNoPrivateKey:
		return Errorf(ClassIdentity, CodeP12NoPrivateKey, "the PKCS#12 file contains no private key")
	case pkcs12ErrNoCertificate:
		return Errorf(ClassIdentity, CodeP12NoCertificate, "the PKCS#12 file contains no certificate")
	}
	return Wrap(ClassIdentity, CodeP12Invalid, err, "the PKCS#12 file could not be decoded")
}

// p12Signer accepts the key types the signing engine supports.
func p12Signer(key any) (crypto.Signer, error) {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		return k, nil
	case *ecdsa.PrivateKey:
		if k.Curve == elliptic.P256() {
			return k, nil
		}
	}
	return nil, Errorf(ClassIdentity, CodeP12Unsupported, "the PKCS#12 private key type is not supported (RSA or ECDSA P-256 required)")
}

// p12IdentityCertificate returns the only non-CA certificate carrying the
// public key of signer, whatever its position in the file.
func p12IdentityCertificate(signer crypto.Signer, certs []*x509.Certificate) (*x509.Certificate, error) {
	var found *x509.Certificate
	for _, cert := range certs {
		if cert.BasicConstraintsValid && cert.IsCA {
			continue
		}
		if !publicKeysEqual(signer.Public(), cert.PublicKey) {
			continue
		}
		if found != nil && !bytes.Equal(found.Raw, cert.Raw) {
			return nil, Errorf(ClassIdentity, CodeP12Ambiguous, "the PKCS#12 file contains several certificates for its private key")
		}
		found = cert
	}
	if found == nil {
		return nil, Errorf(ClassIdentity, CodeP12KeyMismatch, "no certificate of the PKCS#12 file matches its private key")
	}
	return found, nil
}

func publicKeysEqual(a, b crypto.PublicKey) bool {
	key, ok := a.(interface{ Equal(crypto.PublicKey) bool })
	return ok && key.Equal(b)
}
