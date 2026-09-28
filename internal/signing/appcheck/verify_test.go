package appcheck

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/bitxeno/atvloadly/internal/signing"
)

func TestVerifySignedIPA(t *testing.T) {
	signerA, signerB := testSigners(t)
	profileData := []byte("identity provisioning profile")
	input := testIPA(
		appBundle(mainPath, "com.example.app", map[string]any{"application-identifier": "ZZZZZ99999.com.example.app"}),
		appBundle(widgetPath, "com.example.app.widget", map[string]any{}),
	)
	plan := BuildPlan(iPhoneInput(input, testProfile("*", "iOS")))
	removalInput := iPhoneInput(input, testProfile("*", "iOS"))
	removalInput.RemoveExtensions = true
	removalPlan := BuildPlan(removalInput)
	// An explicit profile gives every bundle its application identifier while
	// the bundle identifiers are kept or follow the custom identifier.
	explicitPlan := BuildPlan(iPhoneInput(input, testProfile("com.vendor.slot", "iOS")))
	customInput := iPhoneInput(input, testProfile("com.vendor.slot", "iOS"))
	customInput.CustomIdentifier = "com.vendor.slot"
	customPlan := BuildPlan(customInput)
	for _, p := range []*Plan{plan, removalPlan, explicitPlan, customPlan} {
		if p.Blocking() {
			t.Fatalf("fixture plan is blocking: %+v", p.Issues)
		}
	}

	// faithful returns what the engine produces for p, keyed by bundle path.
	faithful := func(p *Plan) map[string]*fixtureBundle {
		bundles := map[string]*fixtureBundle{}
		for _, b := range p.Bundles {
			if b.Removed {
				continue
			}
			bundles[b.Path] = &fixtureBundle{
				dir:     b.Path,
				id:      b.SignedID,
				sig:     signature{entitlements: cloneDict(b.Entitlements), signer: signerA},
				profile: profileData,
			}
		}
		return bundles
	}
	writeOutput := func(t *testing.T, bundles map[string]*fixtureBundle) string {
		list := make([]fixtureBundle, 0, len(bundles))
		for _, b := range bundles {
			list = append(list, *b)
		}
		return writeIPA(t, bundleFiles(t, list...))
	}

	tests := []struct {
		name string
		// plan is checked against the output (default: the wildcard plan);
		// engine is the plan the engine output is produced for (default: plan).
		plan, engine *Plan
		mutate       func(bundles map[string]*fixtureBundle)
		wantCode     string
		wantBundle   string
	}{
		{name: "faithful output"},
		{name: "faithful output of an explicit profile", plan: explicitPlan},
		{name: "faithful output of a custom identifier", plan: customPlan},
		{name: "faithful output without extensions", plan: removalPlan},
		{
			name: "extension signed with its own application identifier", plan: explicitPlan,
			mutate: func(b map[string]*fixtureBundle) {
				b[widgetPath].sig.entitlements["application-identifier"] = testTeam + ".com.example.app.widget"
			},
			wantCode: signing.CodeSignedEntitlementsInvalid, wantBundle: widgetPath,
		},
		{
			name: "app renamed to the profile App ID", plan: explicitPlan,
			mutate:   func(b map[string]*fixtureBundle) { b[mainPath].id = "com.vendor.slot" },
			wantCode: signing.CodeSignedBundleIdentifierMismatch, wantBundle: mainPath,
		},
		{
			name: "custom identifier not applied to the extension", plan: customPlan,
			mutate:   func(b map[string]*fixtureBundle) { b[widgetPath].id = "com.example.app.widget" },
			wantCode: signing.CodeSignedBundleIdentifierMismatch, wantBundle: widgetPath,
		},
		{
			name:     "other embedded profile",
			mutate:   func(b map[string]*fixtureBundle) { b[widgetPath].profile = []byte("another profile") },
			wantCode: signing.CodeSignedProfileMismatch, wantBundle: widgetPath,
		},
		{
			name:     "no embedded profile",
			mutate:   func(b map[string]*fixtureBundle) { b[mainPath].profile = nil },
			wantCode: signing.CodeSignedProfileMismatch, wantBundle: mainPath,
		},
		{
			name:     "other signer",
			mutate:   func(b map[string]*fixtureBundle) { b[mainPath].sig.signer = signerB },
			wantCode: signing.CodeSignedCertificateMismatch, wantBundle: mainPath,
		},
		{
			name: "ad hoc signature",
			mutate: func(b map[string]*fixtureBundle) {
				b[widgetPath].sig.signer = nil
				b[widgetPath].sig.adHoc = true
			},
			wantCode: signing.CodeSignedCertificateMismatch, wantBundle: widgetPath,
		},
		{
			name:     "other bundle identifier",
			mutate:   func(b map[string]*fixtureBundle) { b[widgetPath].id = "com.example.app.other" },
			wantCode: signing.CodeSignedBundleIdentifierMismatch, wantBundle: widgetPath,
		},
		{
			name:     "changed entitlement value",
			mutate:   func(b map[string]*fixtureBundle) { b[mainPath].sig.entitlements["get-task-allow"] = true },
			wantCode: signing.CodeSignedEntitlementsInvalid, wantBundle: mainPath,
		},
		{
			name: "entitlement missing from the plan",
			mutate: func(b map[string]*fixtureBundle) {
				b[mainPath].sig.entitlements["com.apple.developer.icloud-services"] = []any{"CloudKit"}
			},
			wantCode: signing.CodeSignedEntitlementsInvalid, wantBundle: mainPath,
		},
		{
			name: "wildcard application identifier",
			mutate: func(b map[string]*fixtureBundle) {
				b[widgetPath].sig.entitlements["application-identifier"] = testTeam + ".*"
			},
			wantCode: signing.CodeSignedEntitlementsInvalid, wantBundle: widgetPath,
		},
		{
			name:     "no entitlements",
			mutate:   func(b map[string]*fixtureBundle) { b[mainPath].sig.entitlements = nil },
			wantCode: signing.CodeSignedEntitlementsInvalid, wantBundle: mainPath,
		},
		{
			name:     "missing extension",
			mutate:   func(b map[string]*fixtureBundle) { delete(b, widgetPath) },
			wantCode: signing.CodeSignedBundlesMismatch, wantBundle: widgetPath,
		},
		{
			name: "unexpected extension",
			mutate: func(b map[string]*fixtureBundle) {
				extra := *b[widgetPath]
				extra.dir, extra.id = intentsPath, "com.example.app.intents"
				b[intentsPath] = &extra
			},
			wantCode: signing.CodeSignedBundlesMismatch, wantBundle: intentsPath,
		},
		{
			name:     "removed extension still present",
			plan:     removalPlan,
			engine:   plan,
			wantCode: signing.CodeSignedBundlesMismatch, wantBundle: widgetPath,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checked := plan
			if tt.plan != nil {
				checked = tt.plan
			}
			engine := checked
			if tt.engine != nil {
				engine = tt.engine
			}
			bundles := faithful(engine)
			if tt.mutate != nil {
				tt.mutate(bundles)
			}
			err := VerifySignedIPA(writeOutput(t, bundles), checked, signerA.cert.Raw, profileData)
			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("VerifySignedIPA = %v, want nil", err)
				}
				return
			}
			var signErr *signing.Error
			if !errors.As(err, &signErr) {
				t.Fatalf("VerifySignedIPA = %v, want *signing.Error", err)
			}
			if signErr.Class != signing.ClassSigning || signErr.Code != tt.wantCode {
				t.Fatalf("VerifySignedIPA = %s/%s (%v), want %s/%s", signErr.Class, signErr.Code, err, signing.ClassSigning, tt.wantCode)
			}
			if len(signErr.Issues) != 1 || signErr.Issues[0].Bundle != tt.wantBundle || signErr.Issues[0].Code != tt.wantCode {
				t.Errorf("issues = %+v, want one %s issue for %s", signErr.Issues, tt.wantCode, tt.wantBundle)
			}
		})
	}
}

func TestVerifySignedIPAUnreadableOutput(t *testing.T) {
	plan := BuildPlan(iPhoneInput(testIPA(appBundle(mainPath, "com.example.app", nil)), testProfile("com.example.app", "iOS")))
	for name, path := range map[string]string{
		"missing file": filepath.Join(t.TempDir(), "out.ipa"),
		"directory":    t.TempDir(),
		"not a zip":    writeFile(t, []byte("engine crashed")),
	} {
		err := VerifySignedIPA(path, plan, []byte("cert"), []byte("profile"))
		if signing.ClassOf(err) != signing.ClassSigning || signing.CodeOf(err) != signing.CodeSignedOutputMissing {
			t.Errorf("%s: VerifySignedIPA = %v, want %s", name, err, signing.CodeSignedOutputMissing)
		}
	}
}
