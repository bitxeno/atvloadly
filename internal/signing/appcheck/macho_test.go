package appcheck

import (
	"encoding/binary"
	"testing"

	"github.com/smallstep/pkcs7"
)

// cmsSuperBlob wraps payload as the CMS blob of an embedded signature.
func cmsSuperBlob(payload []byte) []byte {
	const indexEnd = 12 + 8
	out := binary.BigEndian.AppendUint32(nil, csMagicEmbeddedSignature)
	out = binary.BigEndian.AppendUint32(out, uint32(indexEnd+8+len(payload)))
	out = binary.BigEndian.AppendUint32(out, 1)
	out = binary.BigEndian.AppendUint32(out, csSlotSignature)
	out = binary.BigEndian.AppendUint32(out, indexEnd)
	out = binary.BigEndian.AppendUint32(out, csMagicBlobWrapper)
	out = binary.BigEndian.AppendUint32(out, uint32(8+len(payload)))
	return append(out, payload...)
}

func TestMalformedCMSSignatureHasNoSigner(t *testing.T) {
	signer, _ := testSigners(t)
	signed, err := pkcs7.NewSignedData([]byte("synthetic code directory"))
	if err != nil {
		t.Fatalf("cms: %v", err)
	}
	if err := signed.AddSigner(signer.cert, signer.key, pkcs7.SignerInfoConfig{}); err != nil {
		t.Fatalf("cms signer: %v", err)
	}
	signed.Detach()
	cms, err := signed.Finish()
	if err != nil {
		t.Fatalf("cms finish: %v", err)
	}

	payloads := [][]byte{{0x30}, {0x30, 0x82}, {0x1f}, {0x30, 0x80}, {0x30, 0x84, 0xff, 0xff, 0xff, 0xff}}
	for n := 1; n < len(cms); n++ {
		payloads = append(payloads, cms[:n])
	}
	for _, payload := range payloads {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("payload % x... (%d bytes) panics: %v", payload[:min(len(payload), 8)], len(payload), r)
				}
			}()
			if got := cmsSignerSHA256(payload); got != "" {
				t.Errorf("cmsSignerSHA256(%d bytes) = %q, want no certificate", len(payload), got)
			}
			signature, err := parseSuperBlob(cmsSuperBlob(payload))
			if err != nil {
				t.Fatalf("parseSuperBlob(%d bytes CMS): %v", len(payload), err)
			}
			if signature.signerSHA256 != "" {
				t.Errorf("parseSuperBlob(%d bytes CMS) signer = %q, want no certificate", len(payload), signature.signerSHA256)
			}
		}()
	}
	if got := cmsSignerSHA256(cms); got != signer.sha256() {
		t.Errorf("cmsSignerSHA256(complete CMS) = %q, want %q", got, signer.sha256())
	}
}
