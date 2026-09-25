// Package appcheck analyzes an IPA, plans how the signing engine will sign it
// with one provisioning profile, refuses what the engine would mishandle and
// verifies the signed output.
package appcheck

// BundleKind mirrors the engine bundle types.
type BundleKind string

const (
	BundleKindApp          BundleKind = "app"
	BundleKindAppExtension BundleKind = "app_extension"
	BundleKindFramework    BundleKind = "framework"
	BundleKindDylib        BundleKind = "dylib"
)

// Bundle is one bundle found inside the IPA.
type Bundle struct {
	// Path is the zip path of the bundle directory (or dylib file), for
	// example "Payload/App.app/PlugIns/Widget.appex".
	Path               string     `json:"path"`
	Kind               BundleKind `json:"kind"`
	BundleID           string     `json:"bundle_id,omitempty"`
	Executable         string     `json:"executable,omitempty"`
	SupportedPlatforms []string   `json:"supported_platforms,omitempty"`
	PlatformName       string     `json:"platform_name,omitempty"`
	ExtensionPoint     string     `json:"extension_point,omitempty"`
	// Entitlements embedded in the executable code signature; nil when the
	// executable carries none.
	Entitlements map[string]any `json:"-"`
	// SignerCertificateSHA256 is the hex SHA-256 of the certificate that signed
	// the executable CMS blob; "" when unsigned or ad hoc signed.
	SignerCertificateSHA256 string `json:"-"`
	// EmbeddedProfile holds the embedded.mobileprovision bytes; nil when absent.
	EmbeddedProfile []byte `json:"-"`
}

// IPA is the analyzed content of an .ipa file.
type IPA struct {
	Main *Bundle
	// Bundles lists every bundle including Main.
	Bundles []*Bundle

	// unsignedNested lists the App and AppExtension bundle directories nested
	// inside a nested app (for example the extensions of a Watch app). The
	// engine does not descend into nested apps, so it never embeds the
	// provisioning profile nor the profile entitlements into these bundles.
	unsignedNested []string
}

// needsProfile reports whether the engine signs bundles of this kind with the
// provisioning profile and its entitlements.
func (k BundleKind) needsProfile() bool {
	return k == BundleKindApp || k == BundleKindAppExtension
}
