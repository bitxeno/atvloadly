package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bitxeno/atvloadly/internal/signing"
)

// A custom bundle identifier the installation cannot use is refused before
// the installation is queued, whichever signing method is chosen.
func TestInstallAppRefusesUnusableCustomIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		input    installAppInput
		wantErr  string
		wantCode string
	}{
		{
			name:     "apple id",
			input:    installAppInput{IpaURL: "https://example.com/App.ipa", AccountID: "account", CustomIdentifier: "com.example/custom"},
			wantCode: signing.CodeCustomIdentifierInvalid,
		},
		{
			name:     "no signing choice",
			input:    installAppInput{IpaURL: "https://example.com/App.ipa", CustomIdentifier: "com.example/custom"},
			wantCode: signing.CodeCustomIdentifierInvalid,
		},
		{
			name:     "external certificate",
			input:    installAppInput{IpaURL: "https://example.com/App.ipa", SigningIdentityID: 1, CustomIdentifier: "com.example/custom"},
			wantCode: signing.CodeCustomIdentifierInvalid,
		},
		{
			name:     "too long",
			input:    installAppInput{IpaURL: "https://example.com/App.ipa", AccountID: "account", CustomIdentifier: strings.Repeat("a", 256)},
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
			if tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("tool error %q does not contain %q", err, tt.wantErr)
			}
			if tt.wantCode != "" && !strings.HasPrefix(err.Error(), tt.wantCode+": ") {
				t.Fatalf("tool error %q does not start with its code", err)
			}
		})
	}
}

// An Apple ID install accepts a custom bundle identifier: validation passes and
// the call reaches device selection, which is the first step needing a device.
// The tool must not report the identifier as unusable.
func TestInstallAppAppleIDAcceptsCustomIdentifier(t *testing.T) {
	_, _, err := handleInstallApp(context.Background(), nil, installAppInput{
		IpaURL:           "https://example.com/App.ipa",
		AccountID:        "account",
		CustomIdentifier: "com.example.custom",
	})
	if err != nil && strings.Contains(err.Error(), "custom_identifier") {
		t.Fatalf("a valid custom identifier was refused: %v", err)
	}
	if signing.CodeOf(err) == signing.CodeCustomIdentifierInvalid {
		t.Fatalf("a valid custom identifier was refused: %v", err)
	}
}

// The custom bundle identifier is recorded for either signing method: an Apple
// ID install must not drop it, or the queued app would be installed under the
// bundle identifier of the IPA and the second app would never appear.
func TestNewInstallRecordKeepsCustomIdentifier(t *testing.T) {
	device := installDeviceOption{ID: "id", UDID: "DEVICE", Name: "Apple TV", DeviceClass: "AppleTV"}

	for _, tt := range []struct {
		name             string
		input            installAppInput
		customIdentifier string
		want             string
	}{
		{
			name:             "apple id with a custom identifier",
			input:            installAppInput{IpaURL: "https://example.com/App.ipa", AccountID: "account"},
			customIdentifier: "app.custom",
			want:             "app.custom",
		},
		{
			name:  "apple id without a custom identifier",
			input: installAppInput{IpaURL: "https://example.com/App.ipa", AccountID: "account"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := newInstallRecord(tt.input, device, tt.customIdentifier)
			if got.CustomIdentifier != tt.want {
				t.Fatalf("custom identifier = %q, want %q", got.CustomIdentifier, tt.want)
			}
			if got.UDID != device.UDID || got.IpaPath != "https://example.com/App.ipa" {
				t.Fatalf("record not filled from the request: %+v", got)
			}
		})
	}
}
