package appcheck

import (
	"fmt"
	"slices"
	"strings"

	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/signing"
)

// PlanInput is everything BuildPlan needs.
type PlanInput struct {
	IPA     *IPA
	Profile *model.MobileProvisioningProfile
	// DeviceClass selects the required profile and IPA platforms.
	DeviceClass model.DeviceClass
	// DeviceUDIDs are the identifiers of the target device; the device is
	// provisioned when the profile lists any of them.
	DeviceUDIDs []string
	// RemoveExtensions is the explicit user choice to remove app extensions.
	RemoveExtensions bool
	// AllowMissingEntitlements is the explicit user choice to sign even when
	// the signed app will not get every entitlement it requests.
	AllowMissingEntitlements bool
	// CustomIdentifier is the main bundle identifier requested by the user
	// (passed to the engine as --custom-identifier); empty keeps the bundle
	// identifiers of the IPA.
	CustomIdentifier string
}

// PlannedBundle is the expected signing result of one App or AppExtension bundle.
type PlannedBundle struct {
	Path                          string     `json:"path"`
	Kind                          BundleKind `json:"kind"`
	OriginalID                    string     `json:"original_id"`
	SignedID                      string     `json:"signed_id"`
	Removed                       bool       `json:"removed"`
	ExpectedApplicationIdentifier string     `json:"expected_application_identifier,omitempty"`
	// Entitlements the engine will sign the bundle with (simulated).
	Entitlements map[string]any `json:"-"`
}

// Plan is the outcome of BuildPlan.
type Plan struct {
	Issues []signing.Issue `json:"issues"`
	// CustomIdentifier is passed to the engine as --custom-identifier; empty
	// when the bundle identifiers are kept.
	CustomIdentifier   string          `json:"custom_identifier,omitempty"`
	MainBundleID       string          `json:"main_bundle_id"`
	SignedMainBundleID string          `json:"signed_main_bundle_id"`
	Bundles            []PlannedBundle `json:"bundles"`
}

// Blocking reports whether p.Issues contains an error.
func (p *Plan) Blocking() bool {
	return signing.HasErrors(p.Issues)
}

// Err returns nil when p has no blocking issue, else a *signing.Error of
// ClassIdentity with CodeIncompatible carrying p.Issues.
func (p *Plan) Err() error {
	if !p.Blocking() {
		return nil
	}
	return &signing.Error{
		Class:   signing.ClassIdentity,
		Code:    signing.CodeIncompatible,
		Message: "the IPA, the provisioning profile and the device are not compatible",
		Issues:  p.Issues,
	}
}

func (p *Plan) add(code string, severity signing.Severity, bundle string, format string, args ...any) {
	p.Issues = append(p.Issues, signing.Issue{
		Code:     code,
		Severity: severity,
		Message:  fmt.Sprintf(format, args...),
		Bundle:   bundle,
	})
}

// BuildPlan simulates the signing engine for the input and records every finding.
//
// It models PlumeImpactor signing in PEM mode with exactly one --provision:
//   - Bundle identifiers are kept unless a custom identifier is requested:
//     --custom-identifier replaces the main bundle identifier as a substring of
//     the bundle identifier of the main app and of every app extension (an
//     extension "<main>.Widget" becomes "<custom>.Widget").
//   - Every App and AppExtension bundle embeds that profile, whatever its bundle
//     identifier (the engine silently falls back to the first profile when none
//     matches), and is signed with the profile entitlements merged with the
//     entitlements of its executable, every '*' of the profile values being
//     replaced by the signed bundle identifier. An explicit App ID therefore
//     gives every bundle the same application identifier, "TEAM.*" gives each
//     bundle "TEAM.<bundle identifier>" and "TEAM.com.example.*" gives
//     "TEAM.com.example.<bundle identifier>". When the executable embeds no
//     entitlements, the profile entitlements are used verbatim.
//   - --remove-extensions deletes every app extension found below the main app.
//
// A bundle whose application identifier differs from the one its bundle
// identifier would naturally give is a warning, not a refusal: the app runs
// but capabilities bound to the App ID may not work.
func BuildPlan(in PlanInput) *Plan {
	p := &Plan{Issues: []signing.Issue{}, Bundles: []PlannedBundle{}}
	main := in.IPA.Main
	profile := in.Profile
	p.MainBundleID = main.BundleID

	required := in.DeviceClass.ProfilePlatform()
	if required == "" {
		p.add(signing.CodeDevicePlatformUnknown, signing.SeverityError, "",
			"the platform of device class %q is unknown", string(in.DeviceClass))
	} else if !profile.SupportsPlatform(required) {
		p.add(signing.CodeProfilePlatformMismatch, signing.SeverityError, "",
			"the provisioning profile supports %s but the device requires %s", listOrNone(profile.Platform), required)
	}

	platforms := bundlePlatforms(main)
	if len(platforms) == 0 {
		p.add(signing.CodeIPAPlatformUnknown, signing.SeverityWarning, main.Path,
			"the platform the app is built for could not be determined")
	} else if required != "" && !slices.Contains(platforms, required) {
		p.add(signing.CodeIPAPlatformMismatch, signing.SeverityError, main.Path,
			"the app is built for %s but the device requires %s", strings.Join(platforms, ", "), required)
	}

	if !provisionsAny(profile, in.DeviceUDIDs) {
		if len(in.DeviceUDIDs) == 0 {
			p.add(signing.CodeDeviceNotProvisioned, signing.SeverityError, "",
				"the device identifier is unknown, the provisioning profile cannot be checked against it")
		} else {
			p.add(signing.CodeDeviceNotProvisioned, signing.SeverityError, "",
				"the provisioning profile does not include the device %s", strings.Join(in.DeviceUDIDs, ", "))
		}
	}

	profileAppID := profile.ApplicationIdentifier()
	pattern := profile.BundleIDPattern()
	if pattern == "" {
		p.add(signing.CodeApplicationIDMismatch, signing.SeverityError, "",
			"the provisioning profile application identifier %q does not start with its App ID prefix", profileAppID)
	}

	if in.CustomIdentifier != "" {
		if err := ValidateBundleIdentifier(in.CustomIdentifier); err != nil {
			p.add(signing.CodeCustomIdentifierInvalid, signing.SeverityError, "", "%s", err.Error())
		} else if in.CustomIdentifier != main.BundleID {
			p.CustomIdentifier = in.CustomIdentifier
			p.add(signing.CodeBundleIdentifierRewritten, signing.SeverityWarning, main.Path,
				"%s will be installed with the custom identifier %s; the bundle identifiers of its app extensions follow it",
				main.BundleID, in.CustomIdentifier)
		}
	}
	signedID := func(id string) string {
		if p.CustomIdentifier == "" {
			return id
		}
		return strings.ReplaceAll(id, main.BundleID, p.CustomIdentifier)
	}
	p.SignedMainBundleID = signedID(main.BundleID)

	var removedDirs []string
	if in.RemoveExtensions {
		for _, bundle := range in.IPA.Bundles {
			if bundle.Kind == BundleKindAppExtension {
				removedDirs = append(removedDirs, bundle.Path)
			}
		}
	}

	prefix := profile.AppIDPrefix()
	severity := signing.SeverityError
	if in.AllowMissingEntitlements {
		severity = signing.SeverityWarning
	}
	for _, bundle := range in.IPA.Bundles {
		if !bundle.Kind.needsProfile() {
			continue
		}
		planned := PlannedBundle{
			Path:       bundle.Path,
			Kind:       bundle.Kind,
			OriginalID: bundle.BundleID,
			SignedID:   signedID(bundle.BundleID),
		}
		if isBelowAny(bundle.Path, removedDirs) {
			planned.Removed = true
			p.Bundles = append(p.Bundles, planned)
			p.add(signing.CodeExtensionRemoved, signing.SeverityWarning, bundle.Path,
				"%s (%s) will be removed from the app", bundle.Path, bundle.BundleID)
			continue
		}

		planned.Entitlements = simulateEntitlements(profile.Entitlements, bundle.Entitlements, planned.SignedID)
		signedAppID, _ := planned.Entitlements[entitlementApplicationID].(string)
		planned.ExpectedApplicationIdentifier = signedAppID
		natural := prefix + "." + planned.SignedID
		switch {
		case strings.Contains(signedAppID, "*"):
			p.add(signing.CodeWildcardRequiresEntitlements, signing.SeverityError, bundle.Path,
				"the executable of %s embeds no entitlements, so it would be signed with the wildcard application identifier %s instead of one derived from its bundle identifier %s",
				bundle.Path, signedAppID, planned.SignedID)
		case pattern == "":
			// The profile application identifier is already reported as malformed.
		case !allowedByProfile(prefix, pattern, signedAppID):
			p.add(signing.CodeApplicationIDMismatch, signing.SeverityError, bundle.Path,
				"%s would be signed with application identifier %q, which the provisioning profile application identifier %q does not allow",
				bundle.Path, signedAppID, profileAppID)
		case signedAppID != natural:
			p.add(signing.CodeApplicationIdentifierFromProfile, signing.SeverityWarning, bundle.Path,
				"%s (%s) will be signed with application identifier %s derived from the provisioning profile (%s) instead of %s: App Groups, keychain sharing, push notifications and iCloud may not work",
				bundle.Path, planned.SignedID, signedAppID, profileAppID, natural)
		}

		missing, changed := unsatisfiedEntitlements(bundle.Entitlements, planned.Entitlements)
		if len(missing) > 0 {
			p.add(signing.CodeEntitlementsMissing, severity, bundle.Path,
				"the provisioning profile does not grant entitlements requested by %s: %s", bundle.Path, strings.Join(missing, ", "))
		}
		if len(changed) > 0 {
			p.add(signing.CodeEntitlementValueChanged, severity, bundle.Path,
				"%s would be signed with other values than it requests for: %s", bundle.Path, strings.Join(changed, ", "))
		}
		p.Bundles = append(p.Bundles, planned)
	}

	for _, path := range in.IPA.unsignedNested {
		if isBelowAny(path, removedDirs) {
			continue
		}
		p.add(signing.CodeNestedBundlesNotSigned, signing.SeverityError, path,
			"%s is nested inside another app: the signing engine would not sign it with the provisioning profile", path)
	}
	return p
}

// bundlePlatforms returns the profile platform names ("iOS", "tvOS") the bundle
// is built for, from CFBundleSupportedPlatforms or else DTPlatformName.
func bundlePlatforms(bundle *Bundle) []string {
	var platforms []string
	for _, name := range bundle.SupportedPlatforms {
		if platform := profilePlatform(name); platform != "" && !slices.Contains(platforms, platform) {
			platforms = append(platforms, platform)
		}
	}
	if len(platforms) == 0 {
		if platform := profilePlatform(bundle.PlatformName); platform != "" {
			platforms = append(platforms, platform)
		}
	}
	return platforms
}

func profilePlatform(name string) string {
	switch strings.ToLower(name) {
	case "appletvos":
		return "tvOS"
	case "iphoneos":
		return "iOS"
	}
	return ""
}

func provisionsAny(profile *model.MobileProvisioningProfile, udids []string) bool {
	for _, udid := range udids {
		if udid != "" && profile.ProvisionsDevice(udid) {
			return true
		}
	}
	return false
}

// allowedByProfile reports whether appID is an application identifier the
// profile allows: the App ID prefix followed by an identifier that the bundle
// identifier pattern covers. An explicit pattern must be equal; "*" covers
// everything and "prefix.*" covers identifiers that start with "prefix.".
func allowedByProfile(prefix, pattern, appID string) bool {
	id, ok := strings.CutPrefix(appID, prefix+".")
	if !ok {
		return false
	}
	if !strings.Contains(pattern, "*") {
		return id == pattern
	}
	if pattern == "*" {
		return id != ""
	}
	stem, ok := strings.CutSuffix(pattern, "*")
	if !ok || !strings.HasSuffix(stem, ".") || strings.Contains(stem, "*") {
		return false
	}
	return len(id) > len(stem) && strings.HasPrefix(id, stem)
}

func isBelowAny(path string, dirs []string) bool {
	for _, dir := range dirs {
		if path == dir || strings.HasPrefix(path, dir+"/") {
			return true
		}
	}
	return false
}

func listOrNone(values []string) string {
	if len(values) == 0 {
		return "no platform"
	}
	return strings.Join(values, ", ")
}
