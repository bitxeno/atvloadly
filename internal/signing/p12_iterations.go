package signing

import (
	"crypto/x509/pkix"
	"encoding/asn1"
)

// maxP12Iterations bounds every key derivation iteration count of a PKCS#12
// file. go-pkcs12 runs its key derivation loops with the counts read from the
// file, so without a bound a tiny file can pin a CPU forever. Real exports
// use a few thousand iterations; one million keeps a margin while bounding
// the work to well under a second per derivation.
const maxP12Iterations = 1_000_000

var (
	oidP12Data             = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1}
	oidP12EncryptedData    = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 6}
	oidP12ShroudedKeyBag   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 10, 1, 2}
	oidP12PBEWithSHA3DES   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 1, 3}
	oidP12PBEWithSHARC2128 = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 1, 5}
	oidP12PBEWithSHARC240  = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 1, 6}
	oidP12PBES2            = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}
	oidP12PBKDF2           = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 12}
	oidP12PBMAC1           = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 14}
)

// ASN.1 structures equivalent to the ones go-pkcs12 v0.7.3 decodes (RFC 7292,
// RFC 8018 and RFC 9579).

type p12PFX struct {
	Version  int
	AuthSafe p12ContentInfo
	MacData  p12MacData `asn1:"optional"`
}

type p12ContentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"tag:0,explicit,optional"`
}

type p12MacData struct {
	Mac        p12DigestInfo
	MacSalt    []byte
	Iterations int `asn1:"optional,default:1"`
}

type p12DigestInfo struct {
	Algorithm pkix.AlgorithmIdentifier
	Digest    []byte
}

type p12EncryptedData struct {
	Version              int
	EncryptedContentInfo p12EncryptedContentInfo
}

type p12EncryptedContentInfo struct {
	ContentType                asn1.ObjectIdentifier
	ContentEncryptionAlgorithm pkix.AlgorithmIdentifier
	EncryptedContent           []byte `asn1:"tag:0,optional"`
}

type p12SafeBag struct {
	ID         asn1.ObjectIdentifier
	Value      asn1.RawValue  `asn1:"tag:0,explicit"`
	Attributes []p12Attribute `asn1:"set,optional"`
}

type p12Attribute struct {
	ID    asn1.ObjectIdentifier
	Value asn1.RawValue `asn1:"set"`
}

type p12EncryptedPrivateKeyInfo struct {
	AlgorithmIdentifier pkix.AlgorithmIdentifier
	EncryptedData       []byte
}

type p12PBEParams struct {
	Salt       []byte
	Iterations int
}

type p12PBES2Params struct {
	Kdf              pkix.AlgorithmIdentifier
	EncryptionScheme pkix.AlgorithmIdentifier
}

type p12PBMAC1Params struct {
	Kdf    pkix.AlgorithmIdentifier
	MacAlg pkix.AlgorithmIdentifier
}

type p12PBKDF2Params struct {
	Salt       asn1.RawValue
	Iterations int
	KeyLength  int                      `asn1:"optional"`
	Prf        pkix.AlgorithmIdentifier `asn1:"optional"`
}

// checkP12Iterations walks a PKCS#12 file like go-pkcs12 does and rejects it
// with CodeP12Unsupported when an iteration count the library would run its
// key derivation with lies outside 1..maxP12Iterations: the MAC (PKCS#12 KDF
// or PBMAC1), the encryption of encryptedData safe contents and the
// shrouded key bags of plain data safe contents (PBES1 or PBES2).
//
// Only positive detections are rejected: a structure the scan cannot parse is
// skipped, so DecodeChain reports its usual error for it and no file the
// library can decode is refused because of this scan.
//
// Key bags stored inside an encrypted safe contents are only visible after
// decryption; their iteration counts are therefore bounded by the library
// alone.
func checkP12Iterations(data []byte) *Error {
	var pfx p12PFX
	if !p12Unmarshal(data, &pfx) {
		return nil
	}
	if err := checkP12MacIterations(pfx.MacData); err != nil {
		return err
	}
	if !pfx.AuthSafe.ContentType.Equal(oidP12Data) {
		return nil
	}
	var authSafeContent asn1.RawValue
	if !p12Unmarshal(pfx.AuthSafe.Content.Bytes, &authSafeContent) {
		return nil
	}
	var authenticatedSafe []p12ContentInfo
	if !p12Unmarshal(authSafeContent.Bytes, &authenticatedSafe) {
		return nil
	}
	for _, ci := range authenticatedSafe {
		switch {
		case ci.ContentType.Equal(oidP12Data):
			if err := checkP12SafeContentsIterations(ci.Content.Bytes); err != nil {
				return err
			}
		case ci.ContentType.Equal(oidP12EncryptedData):
			var encrypted p12EncryptedData
			if !p12Unmarshal(ci.Content.Bytes, &encrypted) {
				continue
			}
			if err := checkP12PBEIterations(encrypted.EncryptedContentInfo.ContentEncryptionAlgorithm); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkP12MacIterations checks the MAC of a PFX. PBMAC1 takes its iteration
// count from the algorithm parameters and ignores MacData.Iterations.
func checkP12MacIterations(mac p12MacData) *Error {
	algorithm := mac.Mac.Algorithm
	switch {
	case len(algorithm.Algorithm) == 0:
		return nil
	case algorithm.Algorithm.Equal(oidP12PBMAC1):
		var params p12PBMAC1Params
		if !p12Unmarshal(algorithm.Parameters.FullBytes, &params) {
			return nil
		}
		return checkP12PBKDF2Iterations(params.Kdf)
	default:
		return checkP12IterationCount(mac.Iterations)
	}
}

// checkP12SafeContentsIterations checks the shrouded key bags of the plain
// data safe contents whose content is content.
func checkP12SafeContentsIterations(content []byte) *Error {
	var safeContents []byte
	if !p12Unmarshal(content, &safeContents) {
		return nil
	}
	var bags []p12SafeBag
	if !p12Unmarshal(safeContents, &bags) {
		return nil
	}
	for _, bag := range bags {
		if !bag.ID.Equal(oidP12ShroudedKeyBag) {
			continue
		}
		var info p12EncryptedPrivateKeyInfo
		if !p12Unmarshal(bag.Value.Bytes, &info) {
			continue
		}
		if err := checkP12PBEIterations(info.AlgorithmIdentifier); err != nil {
			return err
		}
	}
	return nil
}

// checkP12PBEIterations checks a password based encryption algorithm
// supported by go-pkcs12: PKCS#12 PBES1 or PBES2 with PBKDF2.
func checkP12PBEIterations(algorithm pkix.AlgorithmIdentifier) *Error {
	switch {
	case algorithm.Algorithm.Equal(oidP12PBEWithSHA3DES),
		algorithm.Algorithm.Equal(oidP12PBEWithSHARC2128),
		algorithm.Algorithm.Equal(oidP12PBEWithSHARC240):
		var params p12PBEParams
		if !p12Unmarshal(algorithm.Parameters.FullBytes, &params) {
			return nil
		}
		return checkP12IterationCount(params.Iterations)
	case algorithm.Algorithm.Equal(oidP12PBES2):
		var params p12PBES2Params
		if !p12Unmarshal(algorithm.Parameters.FullBytes, &params) {
			return nil
		}
		return checkP12PBKDF2Iterations(params.Kdf)
	}
	return nil
}

// checkP12PBKDF2Iterations checks the key derivation function of PBES2 or
// PBMAC1 when it is PBKDF2, the only one go-pkcs12 runs.
func checkP12PBKDF2Iterations(kdf pkix.AlgorithmIdentifier) *Error {
	if !kdf.Algorithm.Equal(oidP12PBKDF2) {
		return nil
	}
	var params p12PBKDF2Params
	if !p12Unmarshal(kdf.Parameters.FullBytes, &params) {
		return nil
	}
	return checkP12IterationCount(params.Iterations)
}

func checkP12IterationCount(iterations int) *Error {
	if iterations >= 1 && iterations <= maxP12Iterations {
		return nil
	}
	return Errorf(ClassIdentity, CodeP12Unsupported,
		"the PKCS#12 file uses an iteration count of %d, outside the supported range of 1 to %d", iterations, maxP12Iterations)
}

// p12Unmarshal decodes der into out like go-pkcs12 does, refusing trailing
// data, and reports whether it succeeded.
func p12Unmarshal(der []byte, out any) bool {
	rest, err := asn1.Unmarshal(der, out)
	return err == nil && len(rest) == 0
}
