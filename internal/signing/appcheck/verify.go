package appcheck

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/bitxeno/atvloadly/internal/signing"
)

// VerifySignedIPA checks the engine output against the plan: every kept bundle
// of the plan, and no other App or AppExtension bundle, is present with its
// planned bundle identifier, embeds the identity profile (profileData), is
// signed by the identity certificate (certificateDER) and carries exactly the
// planned application identifier and entitlements, which come from the
// profile and may differ from the ones its bundle identifier would give.
// Errors are *signing.Error of ClassSigning with a CodeSigned* code.
func VerifySignedIPA(path string, plan *Plan, certificateDER, profileData []byte) error {
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return signing.Wrap(signing.ClassSigning, signing.CodeSignedOutputMissing, err,
			"the signing engine did not produce the signed IPA")
	}
	ipa, err := AnalyzeIPA(path)
	if err != nil {
		return signing.Wrap(signing.ClassSigning, signing.CodeSignedOutputMissing, err,
			"the signed IPA could not be read")
	}

	var issues []signing.Issue
	fail := func(code, bundle, format string, args ...any) {
		issues = append(issues, signing.Issue{
			Code:     code,
			Severity: signing.SeverityError,
			Message:  fmt.Sprintf(format, args...),
			Bundle:   bundle,
		})
	}

	signed := map[string]*Bundle{}
	for _, bundle := range ipa.Bundles {
		if bundle.Kind.needsProfile() {
			signed[bundle.Path] = bundle
		}
	}
	planned := map[string]bool{}
	for _, expected := range plan.Bundles {
		if expected.Removed {
			continue
		}
		planned[expected.Path] = true
		if signed[expected.Path] == nil {
			fail(signing.CodeSignedBundlesMismatch, expected.Path, "%s is missing from the signed IPA", expected.Path)
		}
	}
	for _, bundle := range ipa.Bundles {
		if bundle.Kind.needsProfile() && !planned[bundle.Path] {
			fail(signing.CodeSignedBundlesMismatch, bundle.Path, "%s is in the signed IPA but not in the signing plan", bundle.Path)
		}
	}

	sum := sha256.Sum256(certificateDER)
	certificateSHA256 := hex.EncodeToString(sum[:])
	for _, expected := range plan.Bundles {
		bundle := signed[expected.Path]
		if expected.Removed || bundle == nil {
			continue
		}
		if bundle.BundleID != expected.SignedID {
			fail(signing.CodeSignedBundleIdentifierMismatch, expected.Path,
				"%s has bundle identifier %s instead of %s", expected.Path, bundle.BundleID, expected.SignedID)
		}
		if !bytes.Equal(bundle.EmbeddedProfile, profileData) {
			fail(signing.CodeSignedProfileMismatch, expected.Path,
				"%s does not embed the provisioning profile of the signing identity", expected.Path)
		}
		if bundle.SignerCertificateSHA256 == "" || bundle.SignerCertificateSHA256 != certificateSHA256 {
			fail(signing.CodeSignedCertificateMismatch, expected.Path,
				"%s is not signed by the certificate of the signing identity", expected.Path)
		}
		appID, _ := bundle.Entitlements[entitlementApplicationID].(string)
		if appID != expected.ExpectedApplicationIdentifier {
			fail(signing.CodeSignedEntitlementsInvalid, expected.Path,
				"%s is signed with application identifier %q instead of the planned %q", expected.Path, appID, expected.ExpectedApplicationIdentifier)
		} else if bundle.Entitlements == nil || !plistEqual(bundle.Entitlements, expected.Entitlements) {
			fail(signing.CodeSignedEntitlementsInvalid, expected.Path,
				"%s is signed with entitlements that differ from the signing plan", expected.Path)
		}
	}

	if len(issues) == 0 {
		return nil
	}
	return &signing.Error{
		Class:   signing.ClassSigning,
		Code:    issues[0].Code,
		Message: "the signed IPA did not pass verification",
		Issues:  issues,
	}
}
