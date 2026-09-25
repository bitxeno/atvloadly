package signing

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math"
	"testing"
)

// ASN.1 builders for PKCS#12 files declaring arbitrary iteration counts.
var (
	oidTestEncryptedData  = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 6}
	oidTestShroudedKeyBag = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 10, 1, 2}
	oidTestPBEWithSHA3DES = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 1, 3}
	oidTestPBES2          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}
	oidTestPBKDF2         = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 12}
	oidTestPBMAC1         = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 14}
	oidTestHMACWithSHA256 = asn1.ObjectIdentifier{1, 2, 840, 113549, 2, 9}
	oidTestAES256CBC      = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 42}
	oidTestSHA256         = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
)

type testMacPFX struct {
	Version  int
	AuthSafe testContentInfo
	MacData  testMacData
}

type testMacData struct {
	Mac        testDigestInfo
	MacSalt    []byte
	Iterations int
}

type testDigestInfo struct {
	Algorithm pkix.AlgorithmIdentifier
	Digest    []byte
}

type testPBMAC1Params struct {
	Kdf    pkix.AlgorithmIdentifier
	MacAlg pkix.AlgorithmIdentifier
}

type testPBES2Params struct {
	Kdf              pkix.AlgorithmIdentifier
	EncryptionScheme pkix.AlgorithmIdentifier
}

type testPBKDF2Params struct {
	Salt       []byte
	Iterations int
	KeyLength  int
	Prf        pkix.AlgorithmIdentifier
}

type testPBEParams struct {
	Salt       []byte
	Iterations int
}

type testEncryptedData struct {
	Version              int
	EncryptedContentInfo testEncryptedContentInfo
}

type testEncryptedContentInfo struct {
	ContentType                asn1.ObjectIdentifier
	ContentEncryptionAlgorithm pkix.AlgorithmIdentifier
	EncryptedContent           []byte `asn1:"tag:0"`
}

type testEncryptedPrivateKeyInfo struct {
	Algorithm     pkix.AlgorithmIdentifier
	EncryptedData []byte
}

func testAlgorithm(t *testing.T, oid asn1.ObjectIdentifier, params any) pkix.AlgorithmIdentifier {
	t.Helper()
	return pkix.AlgorithmIdentifier{Algorithm: oid, Parameters: asn1.RawValue{FullBytes: mustMarshal(t, params)}}
}

func testNullParameterAlgID(oid asn1.ObjectIdentifier) pkix.AlgorithmIdentifier {
	return pkix.AlgorithmIdentifier{Algorithm: oid, Parameters: asn1.NullRawValue}
}

func testPBKDF2(t *testing.T, iterations int) pkix.AlgorithmIdentifier {
	t.Helper()
	return testAlgorithm(t, oidTestPBKDF2, testPBKDF2Params{
		Salt:       make([]byte, 16),
		Iterations: iterations,
		KeyLength:  32,
		Prf:        testNullParameterAlgID(oidTestHMACWithSHA256),
	})
}

// testDataSafe wraps bags in a plain data safe contents.
func testDataSafe(t *testing.T, bags ...testSafeBag) testContentInfo {
	t.Helper()
	return testContentInfo{ContentType: oidTestData, Content: explicit0(mustMarshal(t, mustMarshal(t, bags)))}
}

// testAuthSafe wraps safes in the authenticated safe of a PFX.
func testAuthSafe(t *testing.T, safes ...testContentInfo) testContentInfo {
	t.Helper()
	return testContentInfo{ContentType: oidTestData, Content: explicit0(mustMarshal(t, mustMarshal(t, safes)))}
}

// testIterationsPFX builds, for a given iteration count, a PKCS#12 file whose
// only key derivation of that kind uses the count.
type testIterationsPFX func(t *testing.T, leaf *x509.Certificate, iterations int) []byte

func TestDecodeP12RejectsIterationCounts(t *testing.T) {
	key := testECKey(t)
	leaf := newTestLeaf(t, key, newTestCA(t, "Synthetic WWDR")).cert

	builders := map[string]testIterationsPFX{
		"MAC": func(t *testing.T, leaf *x509.Certificate, iterations int) []byte {
			return mustMarshal(t, testMacPFX{
				Version:  3,
				AuthSafe: testAuthSafe(t, testDataSafe(t, testCertBagOf(t, leaf), testKeyBag(t, key))),
				MacData: testMacData{
					Mac:        testDigestInfo{Algorithm: testNullParameterAlgID(oidTestSHA256), Digest: make([]byte, 32)},
					MacSalt:    make([]byte, 8),
					Iterations: iterations,
				},
			})
		},
		"PBMAC1": func(t *testing.T, leaf *x509.Certificate, iterations int) []byte {
			params := testPBMAC1Params{Kdf: testPBKDF2(t, iterations), MacAlg: testNullParameterAlgID(oidTestHMACWithSHA256)}
			return mustMarshal(t, testMacPFX{
				Version:  3,
				AuthSafe: testAuthSafe(t, testDataSafe(t, testCertBagOf(t, leaf), testKeyBag(t, key))),
				MacData: testMacData{
					Mac:        testDigestInfo{Algorithm: testAlgorithm(t, oidTestPBMAC1, params), Digest: make([]byte, 32)},
					MacSalt:    make([]byte, 8),
					Iterations: 1,
				},
			})
		},
		"encryptedData PBES2": func(t *testing.T, leaf *x509.Certificate, iterations int) []byte {
			params := testPBES2Params{
				Kdf:              testPBKDF2(t, iterations),
				EncryptionScheme: pkix.AlgorithmIdentifier{Algorithm: oidTestAES256CBC, Parameters: asn1.RawValue{FullBytes: mustMarshal(t, make([]byte, 16))}},
			}
			encrypted := testEncryptedData{EncryptedContentInfo: testEncryptedContentInfo{
				ContentType:                oidTestData,
				ContentEncryptionAlgorithm: testAlgorithm(t, oidTestPBES2, params),
				EncryptedContent:           make([]byte, 32),
			}}
			certs := testContentInfo{ContentType: oidTestEncryptedData, Content: explicit0(mustMarshal(t, encrypted))}
			return mustMarshal(t, testPFX{Version: 3, AuthSafe: testAuthSafe(t, certs, testDataSafe(t, testKeyBag(t, key)))})
		},
		"shrouded key bag PBES1": func(t *testing.T, leaf *x509.Certificate, iterations int) []byte {
			info := testEncryptedPrivateKeyInfo{
				Algorithm:     testAlgorithm(t, oidTestPBEWithSHA3DES, testPBEParams{Salt: make([]byte, 8), Iterations: iterations}),
				EncryptedData: make([]byte, 32),
			}
			keyBag := testSafeBag{ID: oidTestShroudedKeyBag, Value: explicit0(mustMarshal(t, info))}
			return mustMarshal(t, testPFX{Version: 3, AuthSafe: testAuthSafe(t, testDataSafe(t, testCertBagOf(t, leaf), keyBag))})
		},
	}
	for name, build := range builders {
		t.Run(name, func(t *testing.T) {
			// Without the bound, go-pkcs12 would never return for math.MaxInt.
			for _, iterations := range []int{-1, 0, maxP12Iterations + 1, math.MaxInt} {
				_, err := DecodeP12(build(t, leaf, iterations), "")
				if code := testCode(t, err, ClassIdentity); code != CodeP12Unsupported {
					t.Fatalf("%d iterations: code = %q (%v), want %q", iterations, code, err, CodeP12Unsupported)
				}
			}
			// The limit itself is allowed. The scan is called directly so the
			// test does not run a million-iteration key derivation.
			if err := checkP12Iterations(build(t, leaf, maxP12Iterations)); err != nil {
				t.Fatalf("%d iterations rejected: %v", maxP12Iterations, err)
			}
		})
	}
}
