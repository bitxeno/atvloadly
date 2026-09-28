package signing

import (
	"bytes"
	"crypto"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	pkcs12 "software.sslmate.com/src/go-pkcs12"

	"github.com/bitxeno/atvloadly/internal/model"
)

type identityFixture struct {
	pki     *profilePKI
	leaf    *testCert
	p12     []byte
	profile []byte
	sealer  *Sealer
}

const fixturePassword = "fixture-password"

func newIdentityFixture(t *testing.T) *identityFixture {
	t.Helper()
	pki := newProfilePKI(t, profileSignerCommonName)
	ca := newTestCA(t, "Synthetic WWDR")
	leaf := newTestLeaf(t, testRSAKey(t, 1), ca)
	p12, err := pkcs12.Modern2023.Encode(leaf.key, leaf.cert, []*x509.Certificate{ca.cert}, fixturePassword)
	if err != nil {
		t.Fatal(err)
	}
	sealer, err := LoadOrCreateSealer(filepath.Join(t.TempDir(), "signing.key"))
	if err != nil {
		t.Fatal(err)
	}
	return &identityFixture{pki: pki, leaf: leaf, p12: p12, profile: pki.signProfile(t, newTestProfile(leaf.cert)), sealer: sealer}
}

func TestNewIdentity(t *testing.T) {
	f := newIdentityFixture(t)
	identity, issues, err := newIdentity(ImportRequest{
		Name:     "  Living\x00 room\n",
		P12:      f.p12,
		Password: fixturePassword,
		Profile:  f.profile,
	}, f.sealer, testNow, f.pki.roots)
	if err != nil {
		t.Fatal(err)
	}
	if HasErrors(issues) {
		t.Fatalf("unexpected blocking issues %+v", issues)
	}

	cert := f.leaf.cert
	sum256 := sha256.Sum256(cert.Raw)
	sum1 := sha1.Sum(cert.Raw)
	profile := newTestProfile(cert)
	want := model.SigningIdentity{
		Name:                         "Living room",
		CertificateSHA1:              hex.EncodeToString(sum1[:]),
		CertificateSHA256:            hex.EncodeToString(sum256[:]),
		CertificateCommonName:        cert.Subject.CommonName,
		CertificateSerialNumber:      strings.ToUpper(cert.SerialNumber.Text(16)),
		CertificateNotBefore:         testLeafNotBefore,
		CertificateNotAfter:          testLeafNotAfter,
		TeamID:                       testTeamID,
		TeamName:                     profile.TeamName,
		ProfileUUID:                  profile.UUID,
		ProfileName:                  profile.Name,
		ProfileAppIDName:             profile.AppIDName,
		ProfileApplicationIdentifier: testTeamID + ".com.example.app",
		ProfileKind:                  model.ProfileKindAdHoc,
		ProfilePlatforms:             profile.Platform,
		ProfileDeviceCount:           2,
		ProfileCreationDate:          testProfileCreated,
		ProfileExpirationDate:        testProfileExpires,
		Revision:                     1,
	}
	got := *identity
	got.CertificateDER, got.SealedPrivateKey, got.ProfileData = nil, nil, nil
	got.CertificateNotBefore, got.CertificateNotAfter = got.CertificateNotBefore.UTC(), got.CertificateNotAfter.UTC()
	got.ProfileCreationDate, got.ProfileExpirationDate = got.ProfileCreationDate.UTC(), got.ProfileExpirationDate.UTC()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("identity =\n%+v\nwant\n%+v", got, want)
	}
	if !bytes.Equal(identity.CertificateDER, cert.Raw) || !bytes.Equal(identity.ProfileData, f.profile) {
		t.Fatal("certificate or profile bytes not stored verbatim")
	}

	keyDER, err := f.sealer.Open(identity.SealedPrivateKey, IdentityAAD(identity.CertificateSHA256))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(identity.SealedPrivateKey, keyDER) {
		t.Fatal("sealed private key contains the plaintext key")
	}
	key, err := x509.ParsePKCS8PrivateKey(keyDER)
	if err != nil {
		t.Fatal(err)
	}
	if !publicKeysEqual(f.leaf.key.Public(), key.(crypto.Signer).Public()) {
		t.Fatal("sealed key is not the imported key")
	}
}

func TestNewIdentityName(t *testing.T) {
	f := newIdentityFixture(t)
	long := strings.Repeat("é", maxIdentityNameRunes+20)
	tests := []struct {
		name string
		want string
	}{
		{"", f.leaf.cert.Subject.CommonName},
		{" \t\x07 ", f.leaf.cert.Subject.CommonName},
		{long, strings.Repeat("é", maxIdentityNameRunes)},
	}
	for _, tt := range tests {
		identity, _, err := newIdentity(ImportRequest{Name: tt.name, P12: f.p12, Password: fixturePassword, Profile: f.profile}, f.sealer, testNow, f.pki.roots)
		if err != nil {
			t.Fatal(err)
		}
		if identity.Name != tt.want {
			t.Errorf("name %q stored as %q, want %q", tt.name, identity.Name, tt.want)
		}
	}
}

func TestNewIdentityRejectsBlockingIssues(t *testing.T) {
	f := newIdentityFixture(t)
	req := ImportRequest{P12: f.p12, Password: fixturePassword, Profile: f.profile}
	identity, _, err := newIdentity(req, f.sealer, testLeafNotAfter.Add(time.Hour), f.pki.roots)
	if identity != nil {
		t.Fatal("identity returned despite blocking issues")
	}
	var signingErr *Error
	if !errors.As(err, &signingErr) || signingErr.Class != ClassIdentity || signingErr.Code != CodeCertificateExpired {
		t.Fatalf("error = %v, want identity error %s", err, CodeCertificateExpired)
	}
	if !slices.ContainsFunc(signingErr.Issues, func(i Issue) bool { return i.Code == CodeCertificateExpired && i.Severity == SeverityError }) {
		t.Fatalf("error issues %+v lack the blocking finding", signingErr.Issues)
	}
}

func TestReplaceProfile(t *testing.T) {
	f := newIdentityFixture(t)
	identity, _, err := newIdentity(ImportRequest{P12: f.p12, Password: fixturePassword, Profile: f.profile}, f.sealer, testNow, f.pki.roots)
	if err != nil {
		t.Fatal(err)
	}

	next := newTestProfile(f.leaf.cert)
	next.UUID = "5E0C3A1B-0000-4000-8000-000000000099"
	next.ProvisionedDevices = []string{"00008110-000000000000003E"}
	next.ExpirationDate = testProfileExpires.AddDate(0, 1, 0)
	next.TeamName = "Renamed Team"
	nextData := f.pki.signProfile(t, next)

	updated, _, err := replaceProfile(*identity, nextData, testNow, f.pki.roots)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != identity.Revision+1 {
		t.Errorf("revision = %d, want %d", updated.Revision, identity.Revision+1)
	}
	if updated.ProfileUUID != next.UUID || updated.ProfileDeviceCount != 1 || !updated.ProfileExpirationDate.Equal(next.ExpirationDate) || !bytes.Equal(updated.ProfileData, nextData) {
		t.Errorf("profile fields not replaced: %+v", updated)
	}
	if updated.TeamName != next.TeamName {
		t.Errorf("team name = %q, want %q from the replacement profile", updated.TeamName, next.TeamName)
	}
	if updated.CertificateSHA256 != identity.CertificateSHA256 || !bytes.Equal(updated.SealedPrivateKey, identity.SealedPrivateKey) {
		t.Error("certificate or sealed key changed")
	}

	foreign := newTestProfile(newTestLeaf(t, testECKey(t), f.pki.root).cert)
	_, _, err = replaceProfile(*identity, f.pki.signProfile(t, foreign), testNow, f.pki.roots)
	if code := testCode(t, err, ClassIdentity); code != CodeProfileCertificateMissing {
		t.Fatalf("code = %q (%v), want %q", code, err, CodeProfileCertificateMissing)
	}
}

func TestIdentityStatus(t *testing.T) {
	f := newIdentityFixture(t)
	identity, _, err := newIdentity(ImportRequest{P12: f.p12, Password: fixturePassword, Profile: f.profile}, f.sealer, testNow, f.pki.roots)
	if err != nil {
		t.Fatal(err)
	}
	corruptProfile := *identity
	corruptProfile.ProfileData = []byte("corrupt")
	corruptCertificate := *identity
	corruptCertificate.CertificateDER = []byte("corrupt")

	tests := []struct {
		name     string
		identity model.SigningIdentity
		now      time.Time
		want     string
	}{
		{"valid", *identity, testNow, ""},
		{"expired", *identity, testLeafNotAfter.Add(time.Second), CodeCertificateExpired},
		{"corrupt profile", corruptProfile, testNow, CodeProfileInvalid},
		{"corrupt certificate", corruptCertificate, testNow, CodeCertificateInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := IdentityStatus(tt.identity, tt.now)
			got := ""
			for _, issue := range issues {
				if issue.Severity == SeverityError {
					got = issue.Code
					break
				}
			}
			if got != tt.want {
				t.Fatalf("first blocking code = %q, want %q (issues %+v)", got, tt.want, issues)
			}
		})
	}
}
