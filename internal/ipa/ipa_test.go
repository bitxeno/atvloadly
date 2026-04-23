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
	icon := parsed.Icon()
	if icon == nil {
		t.Fatalf("Cannot parse icon from ipa")
	}
}
