package appcheck

import (
	"bytes"
	"io/fs"
	"reflect"
	"slices"
	"testing"

	"github.com/bitxeno/atvloadly/internal/signing"
)

func TestAnalyzeIPACollectsBundlesLikeTheEngine(t *testing.T) {
	signerA, _ := testSigners(t)
	mainEnts := map[string]any{"application-identifier": "OLDTEAM123.com.example.app", "aps-environment": "production"}
	files := bundleFiles(t,
		fixtureBundle{dir: "Payload/Example.app", id: "com.example.app", platforms: []string{"iPhoneOS"},
			sig: signature{entitlements: mainEnts, signer: signerA}, profile: []byte("old profile")},
		fixtureBundle{dir: "Payload/Example.app/PlugIns/Widget.appex", id: "com.example.app.widget",
			platforms: []string{"iPhoneOS"}, extPoint: "com.apple.widgetkit-extension", sig: signature{adHoc: true}},
		// Nested app: collected itself, its content is not.
		fixtureBundle{dir: "Payload/Example.app/Watch/Watch.app", id: "com.example.app.watch", sig: signature{none: true}},
		fixtureBundle{dir: "Payload/Example.app/Watch/Watch.app/PlugIns/WatchExt.appex", id: "com.example.app.watch.ext", sig: signature{none: true}},
	)
	files["Payload/Example.app/Frameworks/Kit.framework/Info.plist"] = []byte("<plist/>")
	files["Payload/Example.app/Frameworks/Kit.framework/Kit"] = machO(t, signature{adHoc: true})
	files["Payload/Example.app/Frameworks/libswift.dylib"] = machO(t, signature{none: true})
	files["Payload/Example.app/Frameworks/notes.dylib"] = []byte("not a Mach-O file")
	// A bundle of unknown type is not signed but its content is walked.
	files["Payload/Example.app/Assets.bundle/Info.plist"] = []byte("<plist/>")
	files["Payload/Example.app/Assets.bundle/Inner.framework/Info.plist"] = []byte("<plist/>")
	// A directory named like a bundle without Info.plist is walked as a plain directory.
	files["Payload/Example.app/Extras/Plain.appex/Deep.appex/Info.plist"] = mustInfoPlist(t, "com.example.app.deep", "Deep")
	files["Payload/Example.app/Extras/Plain.appex/Deep.appex/Deep"] = machO(t, signature{none: true})
	files["iTunesMetadata.plist"] = []byte("metadata")

	ipa, err := AnalyzeIPA(writeIPA(t, files))
	if err != nil {
		t.Fatalf("AnalyzeIPA: %v", err)
	}

	got := map[string]BundleKind{}
	for _, b := range ipa.Bundles {
		got[b.Path] = b.Kind
	}
	want := map[string]BundleKind{
		"Payload/Example.app":                               BundleKindApp,
		"Payload/Example.app/PlugIns/Widget.appex":          BundleKindAppExtension,
		"Payload/Example.app/Watch/Watch.app":               BundleKindApp,
		"Payload/Example.app/Frameworks/Kit.framework":      BundleKindFramework,
		"Payload/Example.app/Frameworks/libswift.dylib":     BundleKindDylib,
		"Payload/Example.app/Assets.bundle/Inner.framework": BundleKindFramework,
		"Payload/Example.app/Extras/Plain.appex/Deep.appex": BundleKindAppExtension,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("bundles = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(ipa.unsignedNested, []string{"Payload/Example.app/Watch/Watch.app/PlugIns/WatchExt.appex"}) {
		t.Errorf("unsignedNested = %v", ipa.unsignedNested)
	}

	main := ipa.Main
	if main != ipa.Bundles[0] || main.BundleID != "com.example.app" || main.Executable != "Example" ||
		!slices.Equal(main.SupportedPlatforms, []string{"iPhoneOS"}) {
		t.Errorf("main bundle = %+v", main)
	}
	if !reflect.DeepEqual(main.Entitlements, mainEnts) {
		t.Errorf("main entitlements = %v, want %v", main.Entitlements, mainEnts)
	}
	if main.SignerCertificateSHA256 != signerA.sha256() {
		t.Errorf("main signer = %q, want %q", main.SignerCertificateSHA256, signerA.sha256())
	}
	if !bytes.Equal(main.EmbeddedProfile, []byte("old profile")) {
		t.Errorf("main embedded profile = %q", main.EmbeddedProfile)
	}

	var widget *Bundle
	for _, b := range ipa.Bundles {
		if b.Path == "Payload/Example.app/PlugIns/Widget.appex" {
			widget = b
		}
	}
	if widget.Entitlements != nil || widget.SignerCertificateSHA256 != "" || widget.EmbeddedProfile != nil {
		t.Errorf("ad hoc widget = entitlements %v, signer %q, profile %q", widget.Entitlements, widget.SignerCertificateSHA256, widget.EmbeddedProfile)
	}
	if widget.ExtensionPoint != "com.apple.widgetkit-extension" {
		t.Errorf("widget extension point = %q", widget.ExtensionPoint)
	}
}

func TestAnalyzeIPAReadsFirstSliceOfUniversalBinary(t *testing.T) {
	signerA, signerB := testSigners(t)
	first := machO(t, signature{entitlements: map[string]any{"slice": "first"}, signer: signerA})
	second := machO(t, signature{entitlements: map[string]any{"slice": "second"}, signer: signerB})
	for _, wide := range []bool{false, true} {
		files := bundleFiles(t, fixtureBundle{dir: "Payload/Example.app", id: "com.example.app"})
		files["Payload/Example.app/Example"] = fatMachO(first, second, wide)
		ipa, err := AnalyzeIPA(writeIPA(t, files))
		if err != nil {
			t.Fatalf("wide=%v: AnalyzeIPA: %v", wide, err)
		}
		if got := ipa.Main.Entitlements["slice"]; got != "first" {
			t.Errorf("wide=%v: entitlements of slice %v, want first", wide, got)
		}
		if ipa.Main.SignerCertificateSHA256 != signerA.sha256() {
			t.Errorf("wide=%v: signer of the wrong slice", wide)
		}
	}
}

func TestAnalyzeIPARejectsInvalidArchives(t *testing.T) {
	valid := func(t *testing.T) map[string][]byte {
		return bundleFiles(t, fixtureBundle{dir: "Payload/Example.app", id: "com.example.app", sig: signature{none: true}})
	}
	infoPlist := mustInfoPlist(t, "com.example.app", "Example")
	tests := []struct {
		name string
		path func(t *testing.T) string
	}{
		{"not a zip", func(t *testing.T) string { return writeFile(t, []byte("not a zip archive")) }},
		{"missing file", func(t *testing.T) string { return t.TempDir() + "/missing.ipa" }},
		{"no Payload", func(t *testing.T) string {
			return writeIPA(t, map[string][]byte{"Example.app/Info.plist": infoPlist})
		}},
		{"two apps", func(t *testing.T) string {
			files := valid(t)
			for name, data := range bundleFiles(t, fixtureBundle{dir: "Payload/Other.app", id: "com.example.other", sig: signature{none: true}}) {
				files[name] = data
			}
			return writeIPA(t, files)
		}},
		{"parent traversal", func(t *testing.T) string {
			files := valid(t)
			files["Payload/Example.app/../../evil"] = []byte("x")
			return writeIPA(t, files)
		}},
		{"absolute path", func(t *testing.T) string {
			files := valid(t)
			files["/etc/evil"] = []byte("x")
			return writeIPA(t, files)
		}},
		{"backslash path", func(t *testing.T) string {
			files := valid(t)
			files["Payload\\..\\evil"] = []byte("x")
			return writeIPA(t, files)
		}},
		{"duplicate entry", func(t *testing.T) string {
			return writeRawZip(t, []rawEntry{
				{name: "Payload/Example.app/Info.plist", data: infoPlist},
				{name: "Payload/Example.app/Info.plist", data: infoPlist},
				{name: "Payload/Example.app/Example", data: machO(t, signature{none: true})},
			})
		}},
		{"file and directory", func(t *testing.T) string {
			files := valid(t)
			files["Payload/Example.app/Example/inner"] = []byte("x")
			return writeIPA(t, files)
		}},
		{"symbolic link", func(t *testing.T) string {
			return writeRawZip(t, []rawEntry{
				{name: "Payload/Example.app/Info.plist", data: infoPlist},
				{name: "Payload/Example.app/Example", data: machO(t, signature{none: true})},
				{name: "Payload/Example.app/link", data: []byte("/etc"), mode: fs.ModeSymlink | 0o777},
			})
		}},
		{"main app without Info.plist", func(t *testing.T) string {
			return writeIPA(t, map[string][]byte{"Payload/Example.app/Example": machO(t, signature{none: true})})
		}},
		{"missing executable", func(t *testing.T) string {
			return writeIPA(t, map[string][]byte{"Payload/Example.app/Info.plist": infoPlist})
		}},
		{"executable outside the bundle", func(t *testing.T) string {
			files := valid(t)
			files["Payload/Example.app/Info.plist"] = mustInfoPlist(t, "com.example.app", "../Example")
			return writeIPA(t, files)
		}},
		{"missing bundle identifier", func(t *testing.T) string {
			files := valid(t)
			files["Payload/Example.app/Info.plist"] = mustInfoPlist(t, "", "Example")
			return writeIPA(t, files)
		}},
		{"oversized Info.plist", func(t *testing.T) string {
			files := valid(t)
			files["Payload/Example.app/Info.plist"] = append(bytes.Repeat([]byte(" "), maxInfoPlistSize), infoPlist...)
			return writeIPA(t, files)
		}},
		{"executable is not Mach-O", func(t *testing.T) string {
			files := valid(t)
			files["Payload/Example.app/Example"] = []byte("#!/bin/sh\necho hello\n")
			return writeIPA(t, files)
		}},
		{"code signature outside the file", func(t *testing.T) string {
			files := valid(t)
			exe := machO(t, signature{adHoc: true})
			files["Payload/Example.app/Example"] = exe[:len(exe)-4]
			return writeIPA(t, files)
		}},
		{"invalid entitlements", func(t *testing.T) string {
			files := valid(t)
			exe := machO(t, signature{entitlements: map[string]any{"key": "value"}})
			// Corrupt the entitlements blob magic.
			at := bytes.Index(exe, []byte{0xfa, 0xde, 0x71, 0x71})
			exe[at+3] = 0x72
			files["Payload/Example.app/Example"] = exe
			return writeIPA(t, files)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := AnalyzeIPA(tt.path(t))
			if signing.ClassOf(err) != signing.ClassSigning || signing.CodeOf(err) != signing.CodeIPAInvalid {
				t.Fatalf("AnalyzeIPA error = %v (class %q, code %q), want %s/%s",
					err, signing.ClassOf(err), signing.CodeOf(err), signing.ClassSigning, signing.CodeIPAInvalid)
			}
		})
	}
}

func mustInfoPlist(t *testing.T, id, executable string) []byte {
	t.Helper()
	info := map[string]any{"CFBundleExecutable": executable}
	if id != "" {
		info["CFBundleIdentifier"] = id
	}
	data, err := plistXML(info)
	if err != nil {
		t.Fatalf("marshal Info.plist: %v", err)
	}
	return data
}
