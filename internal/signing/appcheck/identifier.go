package appcheck

import (
	"regexp"

	"github.com/bitxeno/atvloadly/internal/signing"
)

// maxBundleIdentifierLength bounds a bundle identifier requested by the user.
const maxBundleIdentifierLength = 255

// bundleIdentifierPattern accepts dot separated, non-empty parts made of the
// characters allowed in CFBundleIdentifier: A-Z, a-z, 0-9 and "-".
var bundleIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)*$`)

// ValidateBundleIdentifier checks a bundle identifier requested by the user
// (the custom identifier of an external certificate installation). The error
// is a *signing.Error of ClassSigning with CodeCustomIdentifierInvalid.
func ValidateBundleIdentifier(id string) error {
	switch {
	case id == "":
		return signing.Errorf(signing.ClassSigning, signing.CodeCustomIdentifierInvalid,
			"the custom bundle identifier is empty")
	case len(id) > maxBundleIdentifierLength:
		return signing.Errorf(signing.ClassSigning, signing.CodeCustomIdentifierInvalid,
			"the custom bundle identifier is longer than %d characters", maxBundleIdentifierLength)
	case !bundleIdentifierPattern.MatchString(id):
		return signing.Errorf(signing.ClassSigning, signing.CodeCustomIdentifierInvalid,
			"%q is not a valid bundle identifier: use letters, digits and hyphens in non-empty parts separated by dots", id)
	}
	return nil
}
