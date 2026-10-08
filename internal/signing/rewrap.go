package signing

import (
	"crypto/rand"
	"encoding/hex"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// rewrapPasswordBytes is the entropy of the throwaway password that protects a
// re-wrapped file between this process and the signing engine.
const rewrapPasswordBytes = 16

// RewrapP12 decodes a PKCS#12 file in process and encodes the identity it
// holds again as a legacy PKCS#12 file.
//
// SideStore's "Export Full (.p12)" writes the private key as a plain keyBag
// without a MAC (its export dialog ignores the password it asks for). The
// signing engine reads such a file as holding no private key, so the identity
// is decoded here and written again as a shrouded, MAC-protected file.
//
// The returned password protects the re-wrapped bytes only. It is random, used
// once for that file and never stored, logged or returned to a client. The
// certificate chain of the input is dropped: the signing engine rebuilds the
// chain from the certificate of the account it signs with.
//
// Errors are *Error of ClassIdentity with a CodeP12* code.
func RewrapP12(data []byte, password string) (rewrapped []byte, rewrapPassword string, err error) {
	identity, err := DecodeP12(data, password)
	if err != nil {
		return nil, "", err
	}

	rewrapPassword, err = randomRewrapPassword()
	if err != nil {
		return nil, "", err
	}

	// LegacyDES is the strongest of the two legacy encoders and the shape the
	// engine's reader (p12-keystore) decodes; its modern PBES2 files are
	// refused. See TestRewrapP12.
	rewrapped, err = pkcs12.LegacyDES.Encode(identity.PrivateKey, identity.Certificate, nil, rewrapPassword)
	if err != nil {
		return nil, "", Wrap(ClassIdentity, CodeP12Unsupported, err, "the PKCS#12 file cannot be re-encoded")
	}
	return rewrapped, rewrapPassword, nil
}

// randomRewrapPassword returns a fresh hexadecimal password. Hexadecimal keeps
// the password inside the Unicode BMP, which PKCS#12 password encoding requires.
func randomRewrapPassword() (string, error) {
	raw := make([]byte, rewrapPasswordBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", Wrap(ClassIdentity, CodeKeyUnavailable, err, "a temporary PKCS#12 password cannot be generated")
	}
	defer clear(raw)
	return hex.EncodeToString(raw), nil
}
