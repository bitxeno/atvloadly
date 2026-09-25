package signing

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math"
	"testing"
	"time"
)

// testMaxKDFIterations is the largest iteration count the patched go-pkcs12
// of third_party/go-pkcs12 derives a key with.
const testMaxKDFIterations = 5_000_000

// testKDFIterations is the iteration count of the key derivations the
// builders below compute themselves, like a real export.
const testKDFIterations = 2048

// ASN.1 builders for PKCS#12 files declaring arbitrary algorithms and
// iteration counts.
var (
	oidTestEncryptedData  = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 6}
	oidTestShroudedKeyBag = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 10, 1, 2}
	oidTestPBEWithSHA3DES = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 1, 3}
	oidTestPBES2          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}
	oidTestPBKDF2         = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 12}
	oidTestPBMAC1         = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 14}
	oidTestHMACWithSHA256 = asn1.ObjectIdentifier{1, 2, 840, 113549, 2, 9}
	oidTestHMACWithSHA384 = asn1.ObjectIdentifier{1, 2, 840, 113549, 2, 10}
	oidTestDESEDE3CBC     = asn1.ObjectIdentifier{1, 2, 840, 113549, 3, 7}
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

// testPBKDF2 returns PBKDF2 parameters with a zero 16-byte salt, a 32-byte
// key and prf.
func testPBKDF2(t *testing.T, prf asn1.ObjectIdentifier, iterations int) pkix.AlgorithmIdentifier {
	t.Helper()
	return testAlgorithm(t, oidTestPBKDF2, testPBKDF2Params{
		Salt:       make([]byte, 16),
		Iterations: iterations,
		KeyLength:  32,
		Prf:        testNullParameterAlgID(prf),
	})
}

// testPBKDF2Key derives from password the key of
// testPBKDF2(t, oidTestHMACWithSHA256, testKDFIterations).
func testPBKDF2Key(t *testing.T, password string) []byte {
	t.Helper()
	key, err := pbkdf2.Key(sha256.New, password, make([]byte, 16), testKDFIterations, 32)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// testPBES2 returns a PBES2 algorithm deriving its key with kdf and
// encrypting with the CBC cipher scheme under iv.
func testPBES2(t *testing.T, kdf pkix.AlgorithmIdentifier, scheme asn1.ObjectIdentifier, iv []byte) pkix.AlgorithmIdentifier {
	t.Helper()
	return testAlgorithm(t, oidTestPBES2, testPBES2Params{
		Kdf:              kdf,
		EncryptionScheme: pkix.AlgorithmIdentifier{Algorithm: scheme, Parameters: asn1.RawValue{FullBytes: mustMarshal(t, iv)}},
	})
}

// testPBES2AES256 returns the PBES2 algorithm of testPBES2Encrypt declaring
// iterations: PBKDF2-HMAC-SHA256 and AES-256-CBC with a zero IV.
func testPBES2AES256(t *testing.T, iterations int) pkix.AlgorithmIdentifier {
	t.Helper()
	return testPBES2(t, testPBKDF2(t, oidTestHMACWithSHA256, iterations), oidTestAES256CBC, make([]byte, aes.BlockSize))
}

// testPBES2Encrypt encrypts plaintext under password with
// testPBES2AES256(t, testKDFIterations).
func testPBES2Encrypt(t *testing.T, password string, plaintext []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher(testPBKDF2Key(t, password))
	if err != nil {
		t.Fatal(err)
	}
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := append(bytes.Clone(plaintext), bytes.Repeat([]byte{byte(padding)}, padding)...)
	cipher.NewCBCEncrypter(block, make([]byte, aes.BlockSize)).CryptBlocks(padded, padded)
	return padded
}

// testShroudedKeyBag returns a PKCS#8 shrouded key bag holding encrypted, the
// private key encrypted with algorithm.
func testShroudedKeyBag(t *testing.T, algorithm pkix.AlgorithmIdentifier, encrypted []byte) testSafeBag {
	t.Helper()
	info := testEncryptedPrivateKeyInfo{Algorithm: algorithm, EncryptedData: encrypted}
	return testSafeBag{ID: oidTestShroudedKeyBag, Value: explicit0(mustMarshal(t, info))}
}

// testDataSafe wraps bags in a plain data safe contents.
func testDataSafe(t *testing.T, bags ...testSafeBag) testContentInfo {
	t.Helper()
	return testContentInfo{ContentType: oidTestData, Content: explicit0(mustMarshal(t, mustMarshal(t, bags)))}
}

// testEncryptedSafe returns an encryptedData safe contents holding encrypted,
// the safe bags encrypted with algorithm.
func testEncryptedSafe(t *testing.T, algorithm pkix.AlgorithmIdentifier, encrypted []byte) testContentInfo {
	t.Helper()
	data := testEncryptedData{EncryptedContentInfo: testEncryptedContentInfo{
		ContentType:                oidTestData,
		ContentEncryptionAlgorithm: algorithm,
		EncryptedContent:           encrypted,
	}}
	return testContentInfo{ContentType: oidTestEncryptedData, Content: explicit0(mustMarshal(t, data))}
}

// testAuthSafe wraps safes in the authenticated safe of a PFX.
func testAuthSafe(t *testing.T, safes ...testContentInfo) testContentInfo {
	t.Helper()
	return testContentInfo{ContentType: oidTestData, Content: explicit0(mustMarshal(t, mustMarshal(t, safes)))}
}

// testPFXOf builds a PFX without MAC holding safes.
func testPFXOf(t *testing.T, safes ...testContentInfo) []byte {
	t.Helper()
	return mustMarshal(t, testPFX{Version: 3, AuthSafe: testAuthSafe(t, safes...)})
}

// testPBMAC1PFX builds a PFX holding safes with a PBMAC1 MAC (RFC 9579)
// declaring iterations. The digest is computed for password with
// testKDFIterations, so the MAC only verifies at that count.
func testPBMAC1PFX(t *testing.T, password string, iterations int, safes ...testContentInfo) []byte {
	t.Helper()
	authenticatedSafe := mustMarshal(t, safes)
	mac := hmac.New(sha256.New, testPBKDF2Key(t, password))
	mac.Write(authenticatedSafe)
	params := testPBMAC1Params{Kdf: testPBKDF2(t, oidTestHMACWithSHA256, iterations), MacAlg: testNullParameterAlgID(oidTestHMACWithSHA256)}
	return mustMarshal(t, testMacPFX{
		Version:  3,
		AuthSafe: testContentInfo{ContentType: oidTestData, Content: explicit0(mustMarshal(t, authenticatedSafe))},
		MacData: testMacData{
			Mac:        testDigestInfo{Algorithm: testAlgorithm(t, oidTestPBMAC1, params), Digest: mac.Sum(nil)},
			MacSalt:    make([]byte, 8),
			Iterations: 1,
		},
	})
}

// decodeP12Within runs DecodeP12 and fails the test when it does not return
// within 20 seconds, so an unbounded key derivation fails the test instead of
// hanging it. A stuck decoding goroutine is abandoned.
func decodeP12Within(t *testing.T, data []byte, password string) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := DecodeP12(data, password)
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(20 * time.Second):
		t.Fatal("DecodeP12 did not return within 20s")
		return nil
	}
}

func TestDecodeP12RejectsIterationCounts(t *testing.T) {
	const attackerPassword = "attacker-chosen"

	key := testECKey(t)
	leaf := newTestLeaf(t, key, newTestCA(t, "Synthetic WWDR")).cert
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}

	// nestedSafe returns an encryptedData safe of ordinary parameters whose
	// decrypted contents hold the certificate and a shrouded key bag
	// declaring iterations: that count is only visible after decryption.
	nestedSafe := func(t *testing.T, password string, iterations int) testContentInfo {
		t.Helper()
		keyBag := testShroudedKeyBag(t, testPBES2AES256(t, iterations), testPBES2Encrypt(t, password, pkcs8))
		bags := mustMarshal(t, []testSafeBag{testCertBagOf(t, leaf), keyBag})
		return testEncryptedSafe(t, testPBES2AES256(t, testKDFIterations), testPBES2Encrypt(t, password, bags))
	}

	tests := []struct {
		name     string
		password string
		// build returns a file whose key derivation of the tested kind
		// declares iterations.
		build func(t *testing.T, iterations int) []byte
		// controlCode is the outcome at testKDFIterations: the file is
		// accepted or refused for another reason, so p12_unsupported above
		// the bound comes from the iteration count alone.
		controlCode string
	}{
		{"MAC", "", func(t *testing.T, iterations int) []byte {
			return mustMarshal(t, testMacPFX{
				Version:  3,
				AuthSafe: testAuthSafe(t, testDataSafe(t, testCertBagOf(t, leaf), testKeyBag(t, key))),
				MacData: testMacData{
					Mac:        testDigestInfo{Algorithm: testNullParameterAlgID(oidTestSHA256), Digest: make([]byte, 32)},
					MacSalt:    make([]byte, 8),
					Iterations: iterations,
				},
			})
		}, CodeP12WrongPassword},
		{"PBMAC1", attackerPassword, func(t *testing.T, iterations int) []byte {
			return testPBMAC1PFX(t, attackerPassword, iterations, testDataSafe(t, testCertBagOf(t, leaf), testKeyBag(t, key)))
		}, ""},
		{"encryptedData PBES1", "", func(t *testing.T, iterations int) []byte {
			algorithm := testAlgorithm(t, oidTestPBEWithSHA3DES, testPBEParams{Salt: make([]byte, 8), Iterations: iterations})
			return testPFXOf(t, testEncryptedSafe(t, algorithm, make([]byte, 32)), testDataSafe(t, testKeyBag(t, key)))
		}, CodeP12Invalid},
		{"encryptedData PBES2", "", func(t *testing.T, iterations int) []byte {
			certs := testPBES2Encrypt(t, "", mustMarshal(t, []testSafeBag{testCertBagOf(t, leaf)}))
			return testPFXOf(t, testEncryptedSafe(t, testPBES2AES256(t, iterations), certs), testDataSafe(t, testKeyBag(t, key)))
		}, ""},
		{"shrouded key bag PBES1", "", func(t *testing.T, iterations int) []byte {
			algorithm := testAlgorithm(t, oidTestPBEWithSHA3DES, testPBEParams{Salt: make([]byte, 8), Iterations: iterations})
			return passwordlessPFX(t, testCertBagOf(t, leaf), testShroudedKeyBag(t, algorithm, make([]byte, 32)))
		}, CodeP12Invalid},
		{"shrouded key bag PBES2", "", func(t *testing.T, iterations int) []byte {
			keyBag := testShroudedKeyBag(t, testPBES2AES256(t, iterations), testPBES2Encrypt(t, "", pkcs8))
			return passwordlessPFX(t, testCertBagOf(t, leaf), keyBag)
		}, ""},
		{"shrouded key bag in encryptedData without MAC", "", func(t *testing.T, iterations int) []byte {
			return testPFXOf(t, nestedSafe(t, "", iterations))
		}, ""},
		{"shrouded key bag in encryptedData with PBMAC1", attackerPassword, func(t *testing.T, iterations int) []byte {
			return testPBMAC1PFX(t, attackerPassword, testKDFIterations, nestedSafe(t, attackerPassword, iterations))
		}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := decodeP12Within(t, tt.build(t, testKDFIterations), tt.password)
			if code := testCode(t, err, ClassIdentity); code != tt.controlCode {
				t.Fatalf("%d iterations: code = %q (%v), want %q", testKDFIterations, code, err, tt.controlCode)
			}
			// Without the bound, go-pkcs12 would not return for math.MaxInt.
			for _, iterations := range []int{testMaxKDFIterations + 1, math.MaxInt} {
				err := decodeP12Within(t, tt.build(t, iterations), tt.password)
				if code := testCode(t, err, ClassIdentity); code != CodeP12Unsupported {
					t.Fatalf("%d iterations: code = %q (%v), want %q", iterations, code, err, CodeP12Unsupported)
				}
			}
		})
	}
}
