package signing

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"
)

func TestVerifyProfile(t *testing.T) {
	pki := newProfilePKI(t, profileSignerCommonName)
	leaf := newTestLeaf(t, testECKey(t), newTestCA(t, "Synthetic WWDR"))
	profile := newTestProfile(leaf.cert)
	valid := pki.signProfile(t, profile)

	tampered := bytes.Replace(valid, []byte(profile.UUID), []byte("5E0C3A1B-0000-4000-8000-000000000002"), 1)
	if bytes.Equal(tampered, valid) {
		t.Fatal("tampering did not change the profile")
	}
	wrongSigner := newProfilePKI(t, "Apple iPhone OS Application Signing").signProfile(t, profile)
	untrusted := newProfilePKI(t, profileSignerCommonName).signProfile(t, profile)
	beforeSigner := newTestProfile(leaf.cert)
	beforeSigner.CreationDate = time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC)
	createdBeforeSigner := pki.signProfile(t, beforeSigner)
	noCreationDate := newTestProfile(leaf.cert)
	noCreationDate.CreationDate = time.Time{}

	tests := []struct {
		name     string
		data     []byte
		wantCode string
	}{
		{"valid", valid, ""},
		{"tampered content", tampered, CodeProfileSignatureInvalid},
		{"wrong signer common name", wrongSigner, CodeProfileSignatureInvalid},
		{"untrusted root", untrusted, CodeProfileSignatureInvalid},
		{"signer not valid at creation date", createdBeforeSigner, CodeProfileSignatureInvalid},
		{"no creation date", pki.signProfile(t, noCreationDate), CodeProfileInvalid},
		{"not CMS", []byte("<?xml version=\"1.0\"?><plist></plist>"), CodeProfileInvalid},
		{"signed content is not a profile", pki.signContent(t, []byte("not a property list")), CodeProfileInvalid},
		{"empty", nil, CodeProfileInvalid},
		{"too large", make([]byte, MaxProfileSize+1), CodeUploadTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := verifyProfile(tt.data, pki.roots)
			if code := testCode(t, err, ClassIdentity); code != tt.wantCode {
				t.Fatalf("code = %q (%v), want %q", code, err, tt.wantCode)
			}
			if err == nil && got.UUID != profile.UUID {
				t.Fatalf("UUID = %q, want %q", got.UUID, profile.UUID)
			}
		})
	}
}

// A synthetic profile, however well formed, must not be trusted by the
// exported verifier that only knows the embedded Apple roots.
func TestVerifyProfileTrustsOnlyAppleRoots(t *testing.T) {
	pki := newProfilePKI(t, profileSignerCommonName)
	data := pki.signProfile(t, newTestProfile(newTestLeaf(t, testECKey(t), pki.root).cert))
	_, err := VerifyProfile(data)
	if code := testCode(t, err, ClassIdentity); code != CodeProfileSignatureInvalid {
		t.Fatalf("code = %q (%v), want %q", code, err, CodeProfileSignatureInvalid)
	}
}

// Truncated DER must be refused as an invalid profile, never crash the CMS parser.
func TestVerifyProfileRejectsTruncatedDER(t *testing.T) {
	for _, data := range [][]byte{{0x30}, {0x30, 0x82}, {0x1f}, {0x30, 0x80}} {
		t.Run(hex.EncodeToString(data), func(t *testing.T) {
			_, err := VerifyProfile(data)
			if code := testCode(t, err, ClassIdentity); code != CodeProfileInvalid {
				t.Fatalf("code = %q (%v), want %q", code, err, CodeProfileInvalid)
			}
		})
	}
}

// The embedded roots must match their pinned fingerprints, otherwise every
// real profile would be rejected.
func TestAppleRootPoolPinned(t *testing.T) {
	if _, err := appleRootPool(); err != nil {
		t.Fatal(err)
	}
}
