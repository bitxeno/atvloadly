package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bitxeno/atvloadly/internal/signing"
)

// A custom bundle identifier the installation cannot use is refused before
// the installation is queued.
func TestInstallAppRefusesUnusableCustomIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		input    installAppInput
		wantErr  string
		wantCode string
	}{
		{
			name:    "apple id",
			input:   installAppInput{IpaURL: "https://example.com/App.ipa", AccountID: "account", CustomIdentifier: "com.example.custom"},
			wantErr: "custom_identifier requires signing_identity_id",
		},
		{
			name:    "no signing choice",
			input:   installAppInput{IpaURL: "https://example.com/App.ipa", CustomIdentifier: "com.example.custom"},
			wantErr: "custom_identifier requires signing_identity_id",
		},
		{
			name:     "invalid identifier",
			input:    installAppInput{IpaURL: "https://example.com/App.ipa", SigningIdentityID: 1, CustomIdentifier: "com.example/custom"},
			wantCode: signing.CodeCustomIdentifierInvalid,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, output, err := handleInstallApp(context.Background(), nil, tt.input)
			if err == nil {
				t.Fatalf("install accepted with status %q, want an error", output.Status)
			}
			if signing.CodeOf(err) != tt.wantCode {
				t.Fatalf("signing code = %q, want %q (%v)", signing.CodeOf(err), tt.wantCode, err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("tool error %q does not contain %q", err, tt.wantErr)
			}
			if tt.wantCode != "" && !strings.HasPrefix(err.Error(), tt.wantCode+": ") {
				t.Fatalf("tool error %q does not start with its code", err)
			}
		})
	}
}
