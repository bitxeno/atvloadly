package signing

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"strings"
	"testing"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

func TestDecodeP12(t *testing.T) {
	const password = "S3cret-Pässwörd"

	ca := newTestCA(t, "Synthetic WWDR")
	ecKey := testECKey(t)
	ecLeaf := newTestLeaf(t, ecKey, ca)
	ecLeafReissued := newTestLeaf(t, ecKey, ca)
	rsaKey := testRSAKey(t, 1)
	rsaLeaf := newTestLeaf(t, rsaKey, ca)
	otherKey := testECKey(t)

	p384Key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	p384Leaf := newTestLeaf(t, p384Key, ca)
	_, edKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	edLeaf := newTestLeaf(t, edKey, ca)

	encode := func(enc *pkcs12.Encoder, key any, cert *x509.Certificate, chain []*x509.Certificate, password string) []byte {
		t.Helper()
		data, err := enc.Encode(key, cert, chain, password)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	trustStore, err := pkcs12.Modern2023.EncodeTrustStore([]*x509.Certificate{ecLeaf.cert}, password)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		data     []byte
		password string
		wantCode string
		wantCert *x509.Certificate
	}{
		{"modern with chain", encode(pkcs12.Modern2023, ecKey, ecLeaf.cert, []*x509.Certificate{ca.cert}, password), password, "", ecLeaf.cert},
		{"modern 2026 PBMAC1", encode(pkcs12.Modern2026, ecKey, ecLeaf.cert, []*x509.Certificate{ca.cert}, password), password, "", ecLeaf.cert},
		{"legacy RC2", encode(pkcs12.LegacyRC2, rsaKey, rsaLeaf.cert, nil, password), password, "", rsaLeaf.cert},
		{"legacy 3DES", encode(pkcs12.LegacyDES, rsaKey, rsaLeaf.cert, nil, password), password, "", rsaLeaf.cert},
		{"empty password", encode(pkcs12.Modern2023, ecKey, ecLeaf.cert, nil, ""), "", "", ecLeaf.cert},
		{"passwordless", encode(pkcs12.Passwordless, ecKey, ecLeaf.cert, nil, ""), "", "", ecLeaf.cert},
		{"leaf stored after its CA", encode(pkcs12.Modern2023, ecKey, ca.cert, []*x509.Certificate{ecLeaf.cert}, password), password, "", ecLeaf.cert},
		{"same leaf stored twice", encode(pkcs12.Modern2023, ecKey, ecLeaf.cert, []*x509.Certificate{ecLeaf.cert}, password), password, "", ecLeaf.cert},
		{"incorrect password", encode(pkcs12.Modern2023, ecKey, ecLeaf.cert, nil, password), password + "x", CodeP12WrongPassword, nil},
		{"empty password for protected file", encode(pkcs12.LegacyDES, ecKey, ecLeaf.cert, nil, password), "", CodeP12WrongPassword, nil},
		{"password for passwordless file", encode(pkcs12.Passwordless, ecKey, ecLeaf.cert, nil, ""), password, CodeP12WrongPassword, nil},
		{"invalid bytes", []byte("this is not a PKCS#12 file"), password, CodeP12Invalid, nil},
		{"too large", make([]byte, MaxP12Size+1), password, CodeUploadTooLarge, nil},
		{"no private key", trustStore, password, CodeP12NoPrivateKey, nil},
		{"no certificate", passwordlessPFX(t, testKeyBag(t, ecKey)), "", CodeP12NoCertificate, nil},
		{"key does not match certificate", encode(pkcs12.Modern2023, otherKey, ecLeaf.cert, []*x509.Certificate{ca.cert}, password), password, CodeP12KeyMismatch, nil},
		{"key only matches a CA certificate", encode(pkcs12.Modern2023, ca.key, ca.cert, nil, password), password, CodeP12KeyMismatch, nil},
		{"two certificates for the key", encode(pkcs12.Modern2023, ecKey, ecLeaf.cert, []*x509.Certificate{ecLeafReissued.cert}, password), password, CodeP12Ambiguous, nil},
		{"two key bags", passwordlessPFX(t, testCertBagOf(t, ecLeaf.cert), testKeyBag(t, ecKey), testKeyBag(t, otherKey)), "", CodeP12Ambiguous, nil},
		{"ECDSA P-384 key", encode(pkcs12.Modern2023, p384Key, p384Leaf.cert, nil, password), password, CodeP12Unsupported, nil},
		{"Ed25519 key", encode(pkcs12.Modern2023, edKey, edLeaf.cert, nil, password), password, CodeP12Unsupported, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identity, err := DecodeP12(tt.data, tt.password)
			if code := testCode(t, err, ClassIdentity); code != tt.wantCode {
				t.Fatalf("code = %q (%v), want %q", code, err, tt.wantCode)
			}
			if err != nil {
				if strings.Contains(err.Error(), password) {
					t.Fatalf("error %q leaks the password", err)
				}
				return
			}
			if !identity.Certificate.Equal(tt.wantCert) {
				t.Fatalf("selected certificate %q (serial %s), want serial %s",
					identity.Certificate.Subject.CommonName, identity.Certificate.SerialNumber, tt.wantCert.SerialNumber)
			}
			if !publicKeysEqual(identity.PrivateKey.Public(), tt.wantCert.PublicKey) {
				t.Fatal("private key does not match the selected certificate")
			}
		})
	}
}
