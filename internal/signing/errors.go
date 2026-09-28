// Package signing implements the external certificate signing mode: import of
// a PKCS#12 identity and its provisioning profile, sealed storage of the
// private key, compatibility checks against an IPA and a device, per-task
// materialization of the signing material and verification of the signed
// output produced by PlumeImpactor.
package signing

import (
	"errors"
	"fmt"
	"strings"
)

// Class groups failures by what the user has to fix. Callers map classes to
// model.RefreshedError values and must never retry identity or signing errors.
type Class string

const (
	// ClassIdentity: certificate, private key, provisioning profile, sealing key
	// or their compatibility with the app and the device.
	ClassIdentity Class = "identity"
	// ClassSigning: the IPA could not be read, the signing engine failed or its
	// output did not pass verification.
	ClassSigning Class = "signing"
	// ClassTransport: the device could not be reached or rejected the installation.
	ClassTransport Class = "transport"
)

// Severity of an Issue. Only SeverityError blocks signing.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Issue is one finding of a validation step. Code is stable and machine
// readable; Message is an English diagnostic meant for logs and the UI.
type Issue struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	// Bundle is the bundle path inside the IPA (for example
	// "Payload/App.app/PlugIns/Widget.appex") when the issue concerns one bundle.
	Bundle string `json:"bundle,omitempty"`
}

// Stable issue and error codes. The frontend maps them to translated texts and
// falls back to Issue.Message for unknown codes.
const (
	CodeUploadTooLarge = "upload_too_large"

	CodeP12Invalid       = "p12_invalid"
	CodeP12WrongPassword = "p12_wrong_password"
	CodeP12NoPrivateKey  = "p12_no_private_key"
	CodeP12NoCertificate = "p12_no_certificate"
	CodeP12KeyMismatch   = "p12_key_mismatch"
	CodeP12Ambiguous     = "p12_ambiguous"
	CodeP12Unsupported   = "p12_unsupported"
	// CodeP12PasswordUnsupported: the password contains characters that a
	// PKCS#12 password cannot encode (outside the Unicode BMP, such as emoji).
	CodeP12PasswordUnsupported = "p12_password_unsupported"

	CodeCertificateNotCodeSigning = "certificate_not_code_signing"
	CodeCertificateExpired        = "certificate_expired"
	CodeCertificateNotYetValid    = "certificate_not_yet_valid"
	CodeCertificateNoTeam         = "certificate_no_team"
	CodeRevocationNotChecked      = "revocation_not_checked"

	CodeProfileInvalid            = "profile_invalid"
	CodeProfileSignatureInvalid   = "profile_signature_invalid"
	CodeProfileExpired            = "profile_expired"
	CodeProfileNotInstallable     = "profile_not_installable"
	CodeProfileCertificateMissing = "profile_certificate_mismatch"
	CodeProfileTeamMismatch       = "profile_team_mismatch"
	CodeIdentityExpiringSoon      = "identity_expiring_soon"

	CodeIdentityDuplicate  = "identity_duplicate"
	CodeIdentityNotFound   = "identity_not_found"
	CodeIdentityInUse      = "identity_in_use"
	CodeIdentityReferenced = "identity_referenced"
	CodeKeyUnavailable     = "key_unavailable"
	CodeKeyMismatch        = "key_mismatch"
	CodeSealedKeyCorrupt   = "sealed_key_corrupt"
	CodeWorkspaceFailed    = "workspace_failed"

	CodeDevicePlatformUnknown        = "device_platform_unknown"
	CodeDeviceUnreachable            = "device_unreachable"
	CodeProfilePlatformMismatch      = "profile_platform_mismatch"
	CodeIPAPlatformMismatch          = "ipa_platform_mismatch"
	CodeIPAPlatformUnknown           = "ipa_platform_unknown"
	CodeDeviceNotProvisioned         = "device_not_provisioned"
	CodeIPAInvalid                   = "ipa_invalid"
	CodeIPAPathRejected              = "ipa_path_rejected"
	CodeNestedBundlesNotSigned       = "nested_bundles_not_signed"
	CodeBundleIdentifierRewritten    = "bundle_identifier_rewritten"
	CodeExtensionRemoved             = "extension_removed"
	CodeApplicationIDMismatch        = "application_identifier_mismatch"
	CodeWildcardRequiresEntitlements = "wildcard_requires_binary_entitlements"
	CodeEntitlementsMissing          = "entitlements_missing"
	CodeEntitlementValueChanged      = "entitlement_value_changed"
	CodeIncompatible                 = "incompatible"
	// CodeApplicationIdentifierFromProfile: a bundle is signed with the
	// application identifier of the provisioning profile, which differs from
	// the one its bundle identifier would give (warning).
	CodeApplicationIdentifierFromProfile = "application_identifier_from_profile"
	// CodeCustomIdentifierInvalid: the bundle identifier requested by the user
	// is not a valid bundle identifier.
	CodeCustomIdentifierInvalid = "custom_identifier_invalid"

	CodeEngineFailed                   = "engine_failed"
	CodeSignedOutputMissing            = "signed_output_missing"
	CodeSignedBundlesMismatch          = "signed_bundles_mismatch"
	CodeSignedBundleIdentifierMismatch = "signed_bundle_identifier_mismatch"
	CodeSignedProfileMismatch          = "signed_profile_mismatch"
	CodeSignedCertificateMismatch      = "signed_certificate_mismatch"
	CodeSignedEntitlementsInvalid      = "signed_entitlements_invalid"

	CodeTransportFailed = "transport_failed"
	// CodePairingRecordInvalid: the remote pairing record of the device is
	// missing or unreadable; the device must be paired again.
	CodePairingRecordInvalid = "pairing_record_invalid"

	// CodeCertificateInvalid: the stored certificate of an identity cannot be parsed.
	CodeCertificateInvalid = "certificate_invalid"
)

// Error is the error type returned by this package and by the external
// installation pipeline. Use ClassOf or errors.As to classify it.
type Error struct {
	Class   Class
	Code    string
	Message string
	// Issues holds the detailed findings when the error summarizes a
	// validation step (for example the blocking issues of a compatibility plan).
	Issues []Issue
	Err    error
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(e.Message)
	for _, issue := range e.Issues {
		if issue.Severity != SeverityError {
			continue
		}
		b.WriteString("\n- ")
		b.WriteString(issue.Message)
	}
	if e.Err != nil {
		b.WriteString(": ")
		b.WriteString(e.Err.Error())
	}
	return b.String()
}

func (e *Error) Unwrap() error { return e.Err }

// Errorf builds an *Error without detailed issues.
func Errorf(class Class, code string, format string, args ...any) *Error {
	return &Error{Class: class, Code: code, Message: fmt.Sprintf(format, args...)}
}

// Wrap builds an *Error that keeps cause as its wrapped error.
func Wrap(class Class, code string, cause error, format string, args ...any) *Error {
	return &Error{Class: class, Code: code, Message: fmt.Sprintf(format, args...), Err: cause}
}

// ClassOf returns the class of the first *Error in err's chain, or "" when err
// does not come from the external signing pipeline.
func ClassOf(err error) Class {
	var e *Error
	if errors.As(err, &e) {
		return e.Class
	}
	return ""
}

// CodeOf returns the code of the first *Error in err's chain, or "".
func CodeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// HasErrors reports whether issues contains at least one blocking issue.
func HasErrors(issues []Issue) bool {
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			return true
		}
	}
	return false
}

// IssuesError wraps the blocking issues of a validation step into an *Error of
// the given class, or returns nil when there is no blocking issue.
func IssuesError(class Class, code string, message string, issues []Issue) *Error {
	if !HasErrors(issues) {
		return nil
	}
	return &Error{Class: class, Code: code, Message: message, Issues: issues}
}
