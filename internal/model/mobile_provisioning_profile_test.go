package model

import (
	"encoding/hex"
	"testing"
)

// Truncated DER must be refused with an error, never crash the CMS parser.
func TestParseMobileProvisioningProfileRejectsTruncatedDER(t *testing.T) {
	for _, data := range [][]byte{{0x30}, {0x30, 0x82}, {0x1f}, {0x30, 0x80}} {
		t.Run(hex.EncodeToString(data), func(t *testing.T) {
			if profile, err := ParseMobileProvisioningProfile(data); err == nil {
				t.Fatalf("parsed a profile %+v from truncated DER", profile)
			}
		})
	}
}
