package signing

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/asn1"
	"testing"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// OIDs used to assert the shape of a re-wrapped file.
var (
	oidShroudedKeyBag = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 10, 1, 2}
	oidPlainKeyBag    = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 10, 1, 1}
	oidLocalKeyID     = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 21}
	// oidTripleDESPBES1 is pbeWithSHA1And3-KeyTripleDES-CBC, the algorithm the
	// signing engine's reader decodes. Its PBES2 files are refused.
	oidTripleDESPBES1 = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 1, 3}
)

func oidDER(t *testing.T, oid asn1.ObjectIdentifier) []byte {
	t.Helper()
	der, err := asn1.Marshal(oid)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

// sideStoreShapeP12 builds the file SideStore's "Export Full (.p12)" writes: an
// unencrypted key bag without a MAC holding the key in the clear. Its export
// dialog asks for a password and then ignores it.
func sideStoreShapeP12(t *testing.T, key crypto.Signer, cert *x509.Certificate) []byte {
	t.Helper()
	return passwordlessPFX(t, testCertBagOf(t, cert), testKeyBag(t, key))
}

// A SideStore export must be converted into a shrouded, MAC-protected file the
// signing engine reads, without asking the user for anything else.
func TestRewrapP12ReadsSideStoreExport(t *testing.T) {
	ca := newTestCA(t, "Synthetic WWDR")
	key := testRSAKey(t, 1)
	leaf := newTestLeaf(t, key, ca)

	source := sideStoreShapeP12(t, key, leaf.cert)
	if _, err := DecodeP12(source, ""); err != nil {
		t.Fatalf("the SideStore shape must decode with an empty password: %v", err)
	}

	rewrapped, password, err := RewrapP12(source, "")
	if err != nil {
		t.Fatalf("RewrapP12 refused the SideStore export: %v", err)
	}
	if password == "" {
		t.Fatal("RewrapP12 returned an empty password")
	}

	identity, err := DecodeP12(rewrapped, password)
	if err != nil {
		t.Fatalf("the re-wrapped file does not decode with its own password: %v", err)
	}
	if !bytes.Equal(identity.Certificate.Raw, leaf.cert.Raw) {
		t.Fatal("the re-wrapped file holds another certificate")
	}

	if !bytes.Contains(rewrapped, oidDER(t, oidShroudedKeyBag)) {
		t.Error("the re-wrapped file has no shrouded key bag")
	}
	if bytes.Contains(rewrapped, oidDER(t, oidPlainKeyBag)) {
		t.Error("the re-wrapped file still holds a plain key bag")
	}
	if !bytes.Contains(rewrapped, oidDER(t, oidLocalKeyID)) {
		t.Error("the re-wrapped file has no localKeyId attribute")
	}
	if !bytes.Contains(rewrapped, oidDER(t, oidTripleDESPBES1)) {
		t.Error("the re-wrapped file does not use PBES1 3DES, which the signing engine reads")
	}
	if _, err := DecodeP12(rewrapped, ""); err == nil {
		t.Error("the re-wrapped file opens with an empty password")
	}
}

// A file protected with a password of the user is read with it, and the
// re-wrapped copy is protected with another password.
func TestRewrapP12ReplacesTheInputPassword(t *testing.T) {
	ca := newTestCA(t, "Synthetic WWDR")
	key := testRSAKey(t, 0)
	leaf := newTestLeaf(t, key, ca)

	const userPassword = "S3cret-Pässwörd"
	source, err := pkcs12.LegacyDES.Encode(key, leaf.cert, nil, userPassword)
	if err != nil {
		t.Fatal(err)
	}

	rewrapped, rewrapPassword, err := RewrapP12(source, userPassword)
	if err != nil {
		t.Fatalf("RewrapP12: %v", err)
	}
	if rewrapPassword == userPassword {
		t.Fatal("the re-wrapped file reuses the password of the input")
	}
	if _, err := DecodeP12(rewrapped, rewrapPassword); err != nil {
		t.Fatalf("the re-wrapped file does not open with the returned password: %v", err)
	}
	if _, err := DecodeP12(rewrapped, userPassword); err == nil {
		t.Fatal("the re-wrapped file still opens with the password of the input")
	}

	// The password of the input is still the one that reads the input.
	if _, _, err := RewrapP12(source, "not-the-password"); testCode(t, err, ClassIdentity) != CodeP12WrongPassword {
		t.Fatalf("a wrong input password gave %v", err)
	}
}

// The re-wrap password protects one file: it is fresh for every import.
func TestRewrapP12PasswordIsFresh(t *testing.T) {
	ca := newTestCA(t, "Synthetic WWDR")
	key := testRSAKey(t, 1)
	leaf := newTestLeaf(t, key, ca)
	source := sideStoreShapeP12(t, key, leaf.cert)

	seen := make(map[string]bool)
	for range 8 {
		_, password, err := RewrapP12(source, "")
		if err != nil {
			t.Fatal(err)
		}
		if seen[password] {
			t.Fatalf("the password %q was used for two imports", password)
		}
		seen[password] = true
		// A PKCS#12 password cannot encode characters outside the Unicode BMP.
		for _, r := range password {
			if r > 0xFFFF {
				t.Fatalf("the password %q holds a character outside the BMP", password)
			}
		}
	}
}

// Both key types the signing engine accepts survive a re-wrap.
func TestRewrapP12KeyTypes(t *testing.T) {
	ca := newTestCA(t, "Synthetic WWDR")
	tests := []struct {
		name string
		key  crypto.Signer
	}{
		{name: "RSA", key: testRSAKey(t, 1)},
		{name: "ECDSA P-256", key: testECKey(t)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			leaf := newTestLeaf(t, tc.key, ca)
			rewrapped, password, err := RewrapP12(sideStoreShapeP12(t, tc.key, leaf.cert), "")
			if err != nil {
				t.Fatalf("RewrapP12: %v", err)
			}
			identity, err := DecodeP12(rewrapped, password)
			if err != nil {
				t.Fatalf("decode the re-wrapped file: %v", err)
			}
			if !publicKeysEqual(identity.PrivateKey.Public(), tc.key.Public()) {
				t.Fatal("the re-wrapped file holds another private key")
			}
		})
	}
}

// A file that cannot be used is refused with the code of the decoder.
func TestRewrapP12Errors(t *testing.T) {
	ca := newTestCA(t, "Synthetic WWDR")
	key := testRSAKey(t, 1)
	leaf := newTestLeaf(t, key, ca)
	protected, err := pkcs12.LegacyDES.Encode(key, leaf.cert, nil, "right-password")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		data     []byte
		password string
		wantCode string
	}{
		{name: "empty file", data: nil, wantCode: CodeP12Invalid},
		{name: "not a PKCS#12 file", data: []byte("not a PKCS#12 file"), wantCode: CodeP12Invalid},
		{name: "wrong password", data: protected, password: "wrong-password", wantCode: CodeP12WrongPassword},
		{name: "password for a file without one", data: sideStoreShapeP12(t, key, leaf.cert), password: "unexpected", wantCode: CodeP12WrongPassword},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := RewrapP12(tc.data, tc.password)
			if got := testCode(t, err, ClassIdentity); got != tc.wantCode {
				t.Fatalf("code = %q, want %q", got, tc.wantCode)
			}
		})
	}
}
