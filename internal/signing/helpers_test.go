package signing

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/smallstep/pkcs7"
	plist "howett.net/plist"

	"github.com/bitxeno/atvloadly/internal/model"
)

// Synthetic fixtures. Every certificate, key and profile of these tests is
// generated here; no real Apple material is used.

const testTeamID = "ABCDE12345"

var (
	testNow            = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	testLeafNotBefore  = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	testLeafNotAfter   = time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	testProfileCreated = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	testProfileExpires = time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC)

	testSerial atomic.Int64

	rsaKeyOnce sync.Once
	rsaKeys    [2]*rsa.PrivateKey
)

type testCert struct {
	cert *x509.Certificate
	key  crypto.Signer
}

// testRSAKey returns one of two RSA keys generated once per test binary.
func testRSAKey(t *testing.T, i int) *rsa.PrivateKey {
	t.Helper()
	rsaKeyOnce.Do(func() {
		for n := range rsaKeys {
			key, err := rsa.GenerateKey(rand.Reader, 2048)
			if err != nil {
				panic(err)
			}
			rsaKeys[n] = key
		}
	})
	return rsaKeys[i]
}

func testECKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// issue signs tmpl for key with parent, or self-signs it when parent is nil.
func issue(t *testing.T, tmpl *x509.Certificate, key crypto.Signer, parent *testCert) *testCert {
	t.Helper()
	tmpl.SerialNumber = big.NewInt(testSerial.Add(1) + 0x1000)
	parentCert, parentKey := tmpl, key
	if parent != nil {
		parentCert, parentKey = parent.cert, parent.key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parentCert, key.Public(), parentKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return &testCert{cert: cert, key: key}
}

func caTemplate(cn string) *x509.Certificate {
	return &x509.Certificate{
		Subject:               pkix.Name{CommonName: cn, Organization: []string{"Synthetic CA"}},
		NotBefore:             time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:              time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
}

func leafTemplate() *x509.Certificate {
	return &x509.Certificate{
		Subject: pkix.Name{
			CommonName:         "iPhone Distribution: Synthetic Team (" + testTeamID + ")",
			OrganizationalUnit: []string{testTeamID},
			Organization:       []string{"Synthetic Team"},
		},
		NotBefore:             testLeafNotBefore,
		NotAfter:              testLeafNotAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
	}
}

func newTestCA(t *testing.T, cn string) *testCert {
	t.Helper()
	return issue(t, caTemplate(cn), testECKey(t), nil)
}

// newTestLeaf issues a code signing certificate for key.
func newTestLeaf(t *testing.T, key crypto.Signer, ca *testCert) *testCert {
	t.Helper()
	return issue(t, leafTemplate(), key, ca)
}

// profilePKI mimics Apple's provisioning profile signing chain.
type profilePKI struct {
	root         *testCert
	intermediate *testCert
	signer       *testCert
	roots        *x509.CertPool
}

func newProfilePKI(t *testing.T, signerCN string) *profilePKI {
	t.Helper()
	root := newTestCA(t, "Synthetic Root CA")
	intermediate := issue(t, caTemplate("Synthetic iPhone Certification Authority"), testECKey(t), root)
	signer := issue(t, &x509.Certificate{
		Subject:   pkix.Name{CommonName: signerCN, Organization: []string{"Synthetic Inc."}},
		NotBefore: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:  time.Date(2035, 1, 1, 0, 0, 0, 0, time.UTC),
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}, testRSAKey(t, 0), intermediate)
	roots := x509.NewCertPool()
	roots.AddCert(root.cert)
	return &profilePKI{root: root, intermediate: intermediate, signer: signer, roots: roots}
}

// newTestProfile returns an ad hoc profile of testTeamID listing certs.
func newTestProfile(certs ...*x509.Certificate) *model.MobileProvisioningProfile {
	profile := &model.MobileProvisioningProfile{
		AppIDName:                   "Synthetic App",
		ApplicationIdentifierPrefix: []string{testTeamID},
		CreationDate:                testProfileCreated,
		ExpirationDate:              testProfileExpires,
		Name:                        "Synthetic Ad Hoc",
		Platform:                    []string{"tvOS", "iOS"},
		Entitlements: map[string]any{
			"application-identifier":              testTeamID + ".com.example.app",
			"com.apple.developer.team-identifier": testTeamID,
			"get-task-allow":                      false,
		},
		ProvisionedDevices: []string{"00008110-000000000000001E", "00008110-000000000000002E"},
		TeamIdentifier:     []string{testTeamID},
		TeamName:           "Synthetic Team",
		UUID:               "5E0C3A1B-0000-4000-8000-000000000001",
		Version:            1,
	}
	for _, cert := range certs {
		profile.DeveloperCertificates = append(profile.DeveloperCertificates, cert.Raw)
	}
	return profile
}

// signProfile encodes profile as a CMS signed .mobileprovision.
func (p *profilePKI) signProfile(t *testing.T, profile *model.MobileProvisioningProfile) []byte {
	t.Helper()
	content, err := plist.Marshal(profile, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	return p.signContent(t, content)
}

// signContent wraps content in a CMS signed message of the PKI signer.
func (p *profilePKI) signContent(t *testing.T, content []byte) []byte {
	t.Helper()
	signed, err := pkcs7.NewSignedData(content)
	if err != nil {
		t.Fatal(err)
	}
	if err := signed.AddSigner(p.signer.cert, p.signer.key, pkcs7.SignerInfoConfig{}); err != nil {
		t.Fatal(err)
	}
	signed.AddCertificate(p.intermediate.cert)
	der, err := signed.Finish()
	if err != nil {
		t.Fatal(err)
	}
	return der
}

// testCode returns the code of err, failing when err is not an *Error of class.
func testCode(t *testing.T, err error, class Class) string {
	t.Helper()
	if err == nil {
		return ""
	}
	if got := ClassOf(err); got != class {
		t.Fatalf("error %q has class %q, want %q", err, got, class)
	}
	return CodeOf(err)
}

// ASN.1 builders for hand-made PKCS#12 files the go-pkcs12 encoders cannot produce.
var (
	oidTestData     = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1}
	oidTestKeyBag   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 10, 1, 1}
	oidTestCertBag  = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 10, 1, 3}
	oidTestX509Cert = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 22, 1}
)

type testContentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"tag:0,explicit,optional"`
}

type testSafeBag struct {
	ID    asn1.ObjectIdentifier
	Value asn1.RawValue `asn1:"tag:0,explicit"`
}

type testCertBag struct {
	ID   asn1.ObjectIdentifier
	Data []byte `asn1:"tag:0,explicit"`
}

type testPFX struct {
	Version  int
	AuthSafe testContentInfo
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	der, err := asn1.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

func explicit0(der []byte) asn1.RawValue {
	return asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: der}
}

func testKeyBag(t *testing.T, key crypto.Signer) testSafeBag {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return testSafeBag{ID: oidTestKeyBag, Value: explicit0(der)}
}

func testCertBagOf(t *testing.T, cert *x509.Certificate) testSafeBag {
	t.Helper()
	return testSafeBag{ID: oidTestCertBag, Value: explicit0(mustMarshal(t, testCertBag{ID: oidTestX509Cert, Data: cert.Raw}))}
}

// passwordlessPFX builds an unencrypted PKCS#12 file without MAC holding bags.
func passwordlessPFX(t *testing.T, bags ...testSafeBag) []byte {
	t.Helper()
	safe := testContentInfo{ContentType: oidTestData, Content: explicit0(mustMarshal(t, mustMarshal(t, bags)))}
	authSafe := mustMarshal(t, []testContentInfo{safe})
	return mustMarshal(t, testPFX{Version: 3, AuthSafe: testContentInfo{ContentType: oidTestData, Content: explicit0(mustMarshal(t, authSafe))}})
}
