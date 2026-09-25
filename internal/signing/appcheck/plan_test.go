package appcheck

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/signing"
)

const (
	mainPath    = "Payload/Example.app"
	widgetPath  = "Payload/Example.app/PlugIns/Widget.appex"
	intentsPath = "Payload/Example.app/PlugIns/Intents.appex"
)

type wantIssue struct {
	code     string
	severity signing.Severity
	bundle   string
}

func issuesOf(plan *Plan) []wantIssue {
	got := []wantIssue{}
	for _, issue := range plan.Issues {
		got = append(got, wantIssue{issue.Code, issue.Severity, issue.Bundle})
	}
	return got
}

func appBundle(path, id string, entitlements map[string]any) *Bundle {
	kind := BundleKindApp
	if strings.HasSuffix(path, ".appex") {
		kind = BundleKindAppExtension
	}
	return &Bundle{Path: path, Kind: kind, BundleID: id, SupportedPlatforms: []string{"iPhoneOS"}, Entitlements: entitlements}
}

func testIPA(bundles ...*Bundle) *IPA {
	return &IPA{Main: bundles[0], Bundles: bundles}
}

func iPhoneInput(ipa *IPA, profile *model.MobileProvisioningProfile) PlanInput {
	return PlanInput{IPA: ipa, Profile: profile, DeviceClass: model.DeviceClassiPhone, DeviceUDIDs: []string{testUDID}}
}

func TestBuildPlanBundleIdentifiers(t *testing.T) {
	const (
		slotID     = testTeam + ".com.vendor.slot"
		customID   = "net.custom.app"
		invalidID  = "com.example..app"
		widgetID   = "com.example.app.widget"
		intentsID  = "org.other.intents"
		frameworks = mainPath + "/Frameworks/Kit.framework"
	)
	withWidget := func() *IPA {
		return testIPA(
			appBundle(mainPath, "com.example.app", nil),
			appBundle(widgetPath, widgetID, nil),
			&Bundle{Path: frameworks, Kind: BundleKindFramework},
			appBundle(intentsPath, intentsID, nil),
		)
	}
	slotInput := func(custom string, remove bool) PlanInput {
		in := iPhoneInput(withWidget(), testProfile("com.vendor.slot", "iOS"))
		in.CustomIdentifier = custom
		in.RemoveExtensions = remove
		return in
	}
	keptBundles := []PlannedBundle{
		{Path: mainPath, Kind: BundleKindApp, OriginalID: "com.example.app", SignedID: "com.example.app", ExpectedApplicationIdentifier: slotID},
		{Path: widgetPath, Kind: BundleKindAppExtension, OriginalID: widgetID, SignedID: widgetID, ExpectedApplicationIdentifier: slotID},
		{Path: intentsPath, Kind: BundleKindAppExtension, OriginalID: intentsID, SignedID: intentsID, ExpectedApplicationIdentifier: slotID},
	}
	fromProfileEverywhere := []wantIssue{
		{signing.CodeApplicationIdentifierFromProfile, signing.SeverityWarning, mainPath},
		{signing.CodeApplicationIdentifierFromProfile, signing.SeverityWarning, widgetPath},
		{signing.CodeApplicationIdentifierFromProfile, signing.SeverityWarning, intentsPath},
	}
	tests := []struct {
		name         string
		input        PlanInput
		wantCustom   string
		wantBundles  []PlannedBundle
		wantIssues   []wantIssue
		wantBlocking bool
	}{
		{
			name:  "profile matches the app",
			input: iPhoneInput(testIPA(appBundle(mainPath, "com.example.app", nil)), testProfile("com.example.app", "iOS")),
			wantBundles: []PlannedBundle{
				{Path: mainPath, Kind: BundleKindApp, OriginalID: "com.example.app", SignedID: "com.example.app", ExpectedApplicationIdentifier: testTeam + ".com.example.app"},
			},
			wantIssues: []wantIssue{},
		},
		{
			name:        "explicit profile keeps the identifiers and the extensions",
			input:       slotInput("", false),
			wantBundles: keptBundles,
			wantIssues:  fromProfileEverywhere,
		},
		{
			name:        "custom identifier equal to the app keeps the identifiers",
			input:       slotInput("com.example.app", false),
			wantBundles: keptBundles,
			wantIssues:  fromProfileEverywhere,
		},
		{
			name:       "custom identifier equal to the profile App ID",
			input:      slotInput("com.vendor.slot", false),
			wantCustom: "com.vendor.slot",
			wantBundles: []PlannedBundle{
				{Path: mainPath, Kind: BundleKindApp, OriginalID: "com.example.app", SignedID: "com.vendor.slot", ExpectedApplicationIdentifier: slotID},
				{Path: widgetPath, Kind: BundleKindAppExtension, OriginalID: widgetID, SignedID: "com.vendor.slot.widget", ExpectedApplicationIdentifier: slotID},
				{Path: intentsPath, Kind: BundleKindAppExtension, OriginalID: intentsID, SignedID: intentsID, ExpectedApplicationIdentifier: slotID},
			},
			wantIssues: []wantIssue{
				{signing.CodeBundleIdentifierRewritten, signing.SeverityWarning, mainPath},
				{signing.CodeApplicationIdentifierFromProfile, signing.SeverityWarning, widgetPath},
				{signing.CodeApplicationIdentifierFromProfile, signing.SeverityWarning, intentsPath},
			},
		},
		{
			name:       "custom identifier different from the app and the profile",
			input:      slotInput(customID, false),
			wantCustom: customID,
			wantBundles: []PlannedBundle{
				{Path: mainPath, Kind: BundleKindApp, OriginalID: "com.example.app", SignedID: customID, ExpectedApplicationIdentifier: slotID},
				{Path: widgetPath, Kind: BundleKindAppExtension, OriginalID: widgetID, SignedID: customID + ".widget", ExpectedApplicationIdentifier: slotID},
				{Path: intentsPath, Kind: BundleKindAppExtension, OriginalID: intentsID, SignedID: intentsID, ExpectedApplicationIdentifier: slotID},
			},
			wantIssues: append([]wantIssue{{signing.CodeBundleIdentifierRewritten, signing.SeverityWarning, mainPath}}, fromProfileEverywhere...),
		},
		{
			name:         "invalid custom identifier",
			input:        slotInput(invalidID, false),
			wantBundles:  keptBundles,
			wantIssues:   append([]wantIssue{{signing.CodeCustomIdentifierInvalid, signing.SeverityError, ""}}, fromProfileEverywhere...),
			wantBlocking: true,
		},
		{
			name:  "extensions removed",
			input: slotInput("", true),
			wantBundles: []PlannedBundle{
				keptBundles[0],
				{Path: widgetPath, Kind: BundleKindAppExtension, OriginalID: widgetID, SignedID: widgetID, Removed: true},
				{Path: intentsPath, Kind: BundleKindAppExtension, OriginalID: intentsID, SignedID: intentsID, Removed: true},
			},
			wantIssues: []wantIssue{
				{signing.CodeApplicationIdentifierFromProfile, signing.SeverityWarning, mainPath},
				{signing.CodeExtensionRemoved, signing.SeverityWarning, widgetPath},
				{signing.CodeExtensionRemoved, signing.SeverityWarning, intentsPath},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := BuildPlan(tt.input)
			if plan.CustomIdentifier != tt.wantCustom {
				t.Errorf("CustomIdentifier = %q, want %q", plan.CustomIdentifier, tt.wantCustom)
			}
			if plan.MainBundleID != "com.example.app" || plan.SignedMainBundleID != tt.wantBundles[0].SignedID {
				t.Errorf("main ids = %q -> %q", plan.MainBundleID, plan.SignedMainBundleID)
			}
			got := make([]PlannedBundle, len(plan.Bundles))
			for i, b := range plan.Bundles {
				b.Entitlements = nil
				got[i] = b
			}
			if !reflect.DeepEqual(got, tt.wantBundles) {
				t.Errorf("bundles = %+v\nwant %+v", got, tt.wantBundles)
			}
			if gotIssues := issuesOf(plan); !reflect.DeepEqual(gotIssues, tt.wantIssues) {
				t.Errorf("issues = %+v\nwant %+v", gotIssues, tt.wantIssues)
			}
			if plan.Blocking() != tt.wantBlocking {
				t.Errorf("Blocking() = %v, want %v", plan.Blocking(), tt.wantBlocking)
			}
			err := plan.Err()
			if !tt.wantBlocking {
				if err != nil {
					t.Errorf("Err() = %v, want nil", err)
				}
				return
			}
			var signErr *signing.Error
			if !errors.As(err, &signErr) || signErr.Class != signing.ClassIdentity || signErr.Code != signing.CodeIncompatible ||
				!reflect.DeepEqual(signErr.Issues, plan.Issues) {
				t.Errorf("Err() = %#v, want identity/incompatible error carrying the plan issues", err)
			}
		})
	}
}

func TestBuildPlanWildcardProfileSimulatesEngineEntitlements(t *testing.T) {
	profile := testProfile("*", "iOS")
	ipa := testIPA(
		appBundle(mainPath, "com.example.app", map[string]any{
			"application-identifier": "ZZZZZ99999.com.example.app",
			"keychain-access-groups": []any{"ZZZZZ99999.com.example.app", "com.apple.token"},
		}),
		appBundle(widgetPath, "com.example.app.widget", map[string]any{
			"application-identifier": "ZZZZZ99999.com.example.app.widget",
		}),
	)
	plan := BuildPlan(iPhoneInput(ipa, profile))

	if got := issuesOf(plan); len(got) != 0 {
		t.Fatalf("issues = %+v, want none", got)
	}
	if plan.CustomIdentifier != "" {
		t.Errorf("CustomIdentifier = %q, want none for a wildcard profile", plan.CustomIdentifier)
	}
	want := map[string]map[string]any{
		mainPath: {
			"application-identifier":                 testTeam + ".com.example.app",
			"com.apple.developer.team-identifier":    testTeam,
			"get-task-allow":                         false,
			"keychain-access-groups":                 []any{testTeam + ".com.example.app"},
			"com.apple.security.application-groups":  []any{"group.example.shared", "group.example.cache"},
			"com.apple.developer.associated-domains": "com.example.app",
		},
		widgetPath: {
			"application-identifier":                 testTeam + ".com.example.app.widget",
			"com.apple.developer.team-identifier":    testTeam,
			"get-task-allow":                         false,
			"keychain-access-groups":                 []any{testTeam + ".com.example.app.widget"},
			"com.apple.security.application-groups":  []any{"group.example.shared", "group.example.cache"},
			"com.apple.developer.associated-domains": "com.example.app.widget",
		},
	}
	for _, b := range plan.Bundles {
		if b.ExpectedApplicationIdentifier != testTeam+"."+b.OriginalID || b.SignedID != b.OriginalID {
			t.Errorf("%s: signed %q, expected application identifier %q", b.Path, b.SignedID, b.ExpectedApplicationIdentifier)
		}
		if !reflect.DeepEqual(b.Entitlements, want[b.Path]) {
			t.Errorf("%s entitlements = %#v\nwant %#v", b.Path, b.Entitlements, want[b.Path])
		}
	}
	if got := profile.Entitlements["application-identifier"]; got != testTeam+".*" {
		t.Errorf("BuildPlan modified the profile entitlements: application-identifier = %v", got)
	}
	if got := profile.Entitlements["keychain-access-groups"]; !reflect.DeepEqual(got, []any{testTeam + ".*", "com.apple.token"}) {
		t.Errorf("BuildPlan modified the profile keychain groups: %v", got)
	}
}

func TestBuildPlanWildcardProfiles(t *testing.T) {
	entitled := map[string]any{"application-identifier": "ZZZZZ99999.x"}
	fromProfile := []wantIssue{
		{signing.CodeApplicationIdentifierFromProfile, signing.SeverityWarning, mainPath},
		{signing.CodeApplicationIdentifierFromProfile, signing.SeverityWarning, widgetPath},
	}
	tests := []struct {
		name         string
		pattern      string
		custom       string
		wantAppIDs   []string
		want         []wantIssue
		wantBlocking bool
	}{
		{
			name: "any identifier", pattern: "*",
			wantAppIDs: []string{testTeam + ".com.example.app", testTeam + ".com.example.app.widget"},
			want:       []wantIssue{},
		},
		{
			name: "any identifier with a custom identifier", pattern: "*", custom: "net.custom.app",
			wantAppIDs: []string{testTeam + ".net.custom.app", testTeam + ".net.custom.app.widget"},
			want:       []wantIssue{{signing.CodeBundleIdentifierRewritten, signing.SeverityWarning, mainPath}},
		},
		// The engine replaces '*' with the whole bundle identifier.
		{
			name: "prefix wildcard of another vendor", pattern: "com.vendor.*",
			wantAppIDs: []string{testTeam + ".com.vendor.com.example.app", testTeam + ".com.vendor.com.example.app.widget"},
			want:       fromProfile,
		},
		{
			name: "prefix wildcard covering the app", pattern: "com.example.*",
			wantAppIDs: []string{testTeam + ".com.example.com.example.app", testTeam + ".com.example.com.example.app.widget"},
			want:       fromProfile,
		},
		{
			name: "wildcard inside the App ID", pattern: "com.*.slot",
			wantAppIDs: []string{testTeam + ".com.com.example.app.slot", testTeam + ".com.com.example.app.widget.slot"},
			want: []wantIssue{
				{signing.CodeApplicationIDMismatch, signing.SeverityError, mainPath},
				{signing.CodeApplicationIDMismatch, signing.SeverityError, widgetPath},
			},
			wantBlocking: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ipa := testIPA(appBundle(mainPath, "com.example.app", entitled), appBundle(widgetPath, "com.example.app.widget", entitled))
			in := iPhoneInput(ipa, testProfile(tt.pattern, "iOS"))
			in.CustomIdentifier = tt.custom
			plan := BuildPlan(in)
			if got := issuesOf(plan); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("issues = %+v\nwant %+v", got, tt.want)
			}
			if plan.Blocking() != tt.wantBlocking {
				t.Errorf("Blocking() = %v, want %v", plan.Blocking(), tt.wantBlocking)
			}
			var gotAppIDs []string
			for _, b := range plan.Bundles {
				gotAppIDs = append(gotAppIDs, b.ExpectedApplicationIdentifier)
			}
			if !reflect.DeepEqual(gotAppIDs, tt.wantAppIDs) {
				t.Errorf("expected application identifiers = %q, want %q", gotAppIDs, tt.wantAppIDs)
			}
		})
	}
}

func TestBuildPlanWildcardRequiresEntitlements(t *testing.T) {
	for _, pattern := range []string{"*", "com.vendor.*"} {
		ipa := testIPA(appBundle(mainPath, "com.example.app", nil))
		plan := BuildPlan(iPhoneInput(ipa, testProfile(pattern, "iOS")))
		want := []wantIssue{{signing.CodeWildcardRequiresEntitlements, signing.SeverityError, mainPath}}
		if got := issuesOf(plan); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: issues = %+v, want %+v", pattern, got, want)
		}
		// Without embedded entitlements the engine signs with the profile verbatim.
		if got := plan.Bundles[0].Entitlements["application-identifier"]; got != testTeam+"."+pattern {
			t.Errorf("%s: simulated application-identifier = %v, want the wildcard kept", pattern, got)
		}
	}
}

func TestBuildPlanApplicationIdentifierWithoutAppIDPrefix(t *testing.T) {
	profile := testProfile("com.example.app", "iOS")
	profile.ApplicationIdentifierPrefix = []string{"OTHER12345"}
	profile.TeamIdentifier = []string{"OTHER12345"}
	ipa := testIPA(appBundle(mainPath, "com.example.app", map[string]any{"application-identifier": "ZZZZZ99999.x"}))
	plan := BuildPlan(iPhoneInput(ipa, profile))
	want := []wantIssue{{signing.CodeApplicationIDMismatch, signing.SeverityError, ""}}
	if got := issuesOf(plan); !reflect.DeepEqual(got, want) {
		t.Errorf("issues = %+v, want %+v", got, want)
	}
}

func TestBuildPlanRequestedEntitlements(t *testing.T) {
	tests := []struct {
		name      string
		requested map[string]any
		allow     bool
		want      []wantIssue
	}{
		{
			name: "only engine managed keys",
			requested: map[string]any{
				"application-identifier":              "ZZZZZ99999.com.example.app",
				"com.apple.developer.team-identifier": "ZZZZZ99999",
				"keychain-access-groups":              []any{"ZZZZZ99999.com.example.app"},
				"get-task-allow":                      true,
			},
			want: []wantIssue{},
		},
		{
			name:      "granted subset",
			requested: map[string]any{"com.apple.security.application-groups": []any{"group.example.cache"}},
			want:      []wantIssue{},
		},
		{
			name:      "missing entitlement",
			requested: map[string]any{"com.apple.developer.healthkit": true},
			want:      []wantIssue{{signing.CodeEntitlementsMissing, signing.SeverityError, mainPath}},
		},
		{
			name:      "missing entitlement allowed",
			requested: map[string]any{"com.apple.developer.healthkit": true},
			allow:     true,
			want:      []wantIssue{{signing.CodeEntitlementsMissing, signing.SeverityWarning, mainPath}},
		},
		{
			name:      "app group not granted",
			requested: map[string]any{"com.apple.security.application-groups": []any{"group.example.shared", "group.other"}},
			want:      []wantIssue{{signing.CodeEntitlementValueChanged, signing.SeverityError, mainPath}},
		},
		{
			name: "missing and changed allowed",
			requested: map[string]any{
				"com.apple.security.application-groups":  []any{"group.other"},
				"com.apple.developer.associated-domains": []any{"applinks:example.com"},
				"aps-environment":                        "production",
			},
			allow: true,
			want: []wantIssue{
				{signing.CodeEntitlementsMissing, signing.SeverityWarning, mainPath},
				{signing.CodeEntitlementValueChanged, signing.SeverityWarning, mainPath},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := iPhoneInput(testIPA(appBundle(mainPath, "com.example.app", tt.requested)), testProfile("com.example.app", "iOS"))
			in.AllowMissingEntitlements = tt.allow
			plan := BuildPlan(in)
			if got := issuesOf(plan); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("issues = %+v\nwant %+v", got, tt.want)
			}
			if plan.Blocking() == tt.allow && len(tt.want) > 0 {
				t.Errorf("Blocking() = %v with AllowMissingEntitlements = %v", plan.Blocking(), tt.allow)
			}
		})
	}
}

func TestBuildPlanDeviceAndPlatform(t *testing.T) {
	iosApp := func() *Bundle { return appBundle(mainPath, "com.example.app", nil) }
	tvApp := func() *Bundle {
		b := appBundle(mainPath, "com.example.app", nil)
		b.SupportedPlatforms = []string{"AppleTVOS"}
		return b
	}
	tests := []struct {
		name    string
		main    *Bundle
		profile *model.MobileProvisioningProfile
		class   model.DeviceClass
		udids   []string
		want    []wantIssue
	}{
		{
			name: "iOS profile and app on an Apple TV", main: iosApp(), profile: testProfile("com.example.app", "iOS", "xrOS"),
			class: model.DeviceClassAppleTV, udids: []string{testUDID},
			want: []wantIssue{
				{signing.CodeProfilePlatformMismatch, signing.SeverityError, ""},
				{signing.CodeIPAPlatformMismatch, signing.SeverityError, mainPath},
			},
		},
		{
			name: "tvOS profile and app on an Apple TV", main: tvApp(), profile: testProfile("com.example.app", "tvOS"),
			class: model.DeviceClassAppleTV, udids: []string{testUDID}, want: []wantIssue{},
		},
		{
			name: "platform from DTPlatformName", main: func() *Bundle {
				b := appBundle(mainPath, "com.example.app", nil)
				b.SupportedPlatforms, b.PlatformName = nil, "appletvos"
				return b
			}(), profile: testProfile("com.example.app", "tvOS"),
			class: model.DeviceClassAppleTV, udids: []string{testUDID}, want: []wantIssue{},
		},
		{
			name: "tvOS app on an iPad", main: tvApp(), profile: testProfile("com.example.app", "iOS"),
			class: model.DeviceClassiPad, udids: []string{testUDID},
			want: []wantIssue{{signing.CodeIPAPlatformMismatch, signing.SeverityError, mainPath}},
		},
		{
			name: "unknown app platform", main: func() *Bundle {
				b := appBundle(mainPath, "com.example.app", nil)
				b.SupportedPlatforms = []string{"iPhoneSimulator"}
				return b
			}(), profile: testProfile("com.example.app", "iOS"),
			class: model.DeviceClassiPhone, udids: []string{testUDID},
			want: []wantIssue{{signing.CodeIPAPlatformUnknown, signing.SeverityWarning, mainPath}},
		},
		{
			name: "unknown device class", main: iosApp(), profile: testProfile("com.example.app", "iOS"),
			class: "", udids: []string{testUDID},
			want: []wantIssue{{signing.CodeDevicePlatformUnknown, signing.SeverityError, ""}},
		},
		{
			name: "device not in the profile", main: iosApp(), profile: testProfile("com.example.app", "iOS"),
			class: model.DeviceClassiPhone, udids: []string{"00008120-00000000000000FF"},
			want: []wantIssue{{signing.CodeDeviceNotProvisioned, signing.SeverityError, ""}},
		},
		{
			name: "device identifier unknown", main: iosApp(), profile: testProfile("com.example.app", "iOS"),
			class: model.DeviceClassiPhone,
			want:  []wantIssue{{signing.CodeDeviceNotProvisioned, signing.SeverityError, ""}},
		},
		{
			name: "any device identifier listed", main: iosApp(), profile: testProfile("com.example.app", "iOS"),
			class: model.DeviceClassiPhone, udids: []string{"serial-number", strings.ToLower(testUDID)},
			want: []wantIssue{},
		},
		{
			name: "profile provisioning all devices", main: iosApp(), profile: func() *model.MobileProvisioningProfile {
				p := testProfile("com.example.app", "iOS")
				p.ProvisionedDevices, p.ProvisionsAllDevices = nil, true
				return p
			}(),
			class: model.DeviceClassiPhone, udids: []string{"00008120-00000000000000FF"}, want: []wantIssue{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := BuildPlan(PlanInput{IPA: testIPA(tt.main), Profile: tt.profile, DeviceClass: tt.class, DeviceUDIDs: tt.udids})
			if got := issuesOf(plan); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("issues = %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestBuildPlanRejectsBundlesNestedInNestedApps(t *testing.T) {
	const watchExt = mainPath + "/Watch/Watch.app/PlugIns/WatchExt.appex"
	ipa := testIPA(
		appBundle(mainPath, "com.example.app", nil),
		appBundle(mainPath+"/Watch/Watch.app", "com.example.app.watchkitapp", nil),
	)
	ipa.unsignedNested = []string{watchExt}
	for _, remove := range []bool{false, true} {
		in := iPhoneInput(ipa, testProfile("*", "iOS"))
		in.RemoveExtensions = remove
		plan := BuildPlan(in)
		found := false
		for _, issue := range plan.Issues {
			if issue.Code == signing.CodeNestedBundlesNotSigned && issue.Bundle == watchExt && issue.Severity == signing.SeverityError {
				found = true
			}
		}
		if !found {
			t.Errorf("RemoveExtensions=%v: issues = %+v, want %s for %s", remove, plan.Issues, signing.CodeNestedBundlesNotSigned, watchExt)
		}
	}
}
