package ipa

import (
	"testing"
)

func TestParseFile(t *testing.T) {
	ipaPath := "/path/to/xxxx.ipa"

	parsed, err := ParseFile(ipaPath)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	if got, want := parsed.Name(), "Test App"; got != want {
		t.Fatalf("Name() = %q, want %q", got, want)
	}
	if got, want := parsed.Identifier(), "com.example.testapp"; got != want {
		t.Fatalf("Identifier() = %q, want %q", got, want)
	}
	if got, want := parsed.Version(), "1.2.3"; got != want {
		t.Fatalf("Version() = %q, want %q", got, want)
	}
}
