package appcheck

import (
	"strings"
	"testing"

	"github.com/bitxeno/atvloadly/internal/signing"
)

func TestValidateBundleIdentifier(t *testing.T) {
	valid := []string{
		"com.example.app",
		"com.example-app.v2",
		"a",
		"123.456",
		"A-B.c-D",
		"-.-",
		strings.Repeat("a", 255),
		strings.Repeat("a.", 127) + "a",
	}
	for _, id := range valid {
		if err := ValidateBundleIdentifier(id); err != nil {
			t.Errorf("ValidateBundleIdentifier(%q) = %v, want nil", id, err)
		}
	}

	invalid := []string{
		"",
		strings.Repeat("a", 256),
		strings.Repeat("a.", 127) + "ab",
		"com..app",
		".com.app",
		"com.app.",
		".",
		"com.example.*",
		"*",
		"com.example app",
		" com.example.app",
		"com.example_app",
		"com/example/app",
		"com.exämple.app",
	}
	for _, id := range invalid {
		err := ValidateBundleIdentifier(id)
		if signing.ClassOf(err) != signing.ClassSigning || signing.CodeOf(err) != signing.CodeCustomIdentifierInvalid {
			t.Errorf("ValidateBundleIdentifier(%q) = %v, want %s/%s", id, err, signing.ClassSigning, signing.CodeCustomIdentifierInvalid)
		}
	}
}
