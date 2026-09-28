package appcheck

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"

	"howett.net/plist"

	"github.com/bitxeno/atvloadly/internal/signing"
)

// Limits applied while reading an IPA. The archive is never extracted and no
// executable is ever buffered as a whole.
const (
	maxZipEntries       = 200_000
	maxInfoPlistSize    = 1 << 20
	maxEmbeddedProfile  = 2 << 20
	maxCodeSignature    = 16 << 20
	maxLoadCommandsSize = 4 << 20
	maxFatArchs         = 64
)

const (
	payloadDirName   = "Payload"
	infoPlistName    = "Info.plist"
	embeddedProfName = "embedded.mobileprovision"
)

// infoPlist holds the Info.plist keys the engine and the checks rely on.
type infoPlist struct {
	CFBundleIdentifier         string   `plist:"CFBundleIdentifier"`
	CFBundleExecutable         string   `plist:"CFBundleExecutable"`
	CFBundleSupportedPlatforms []string `plist:"CFBundleSupportedPlatforms"`
	DTPlatformName             string   `plist:"DTPlatformName"`
	NSExtension                struct {
		NSExtensionPointIdentifier string `plist:"NSExtensionPointIdentifier"`
	} `plist:"NSExtension"`
}

// node is one file or directory of the archive tree.
type node struct {
	name     string
	path     string
	dir      bool
	file     *zip.File // nil for directories
	children map[string]*node
}

func (n *node) sortedChildren() []*node {
	children := make([]*node, 0, len(n.children))
	for _, child := range n.children {
		children = append(children, child)
	}
	sort.Slice(children, func(i, j int) bool { return children[i].name < children[j].name })
	return children
}

// hasChild reports whether the directory contains an entry named name, like
// the engine's Path::exists check.
func (n *node) hasChild(name string) bool {
	return n.dir && n.children[name] != nil
}

func invalidIPA(format string, args ...any) *signing.Error {
	return signing.Errorf(signing.ClassSigning, signing.CodeIPAInvalid, "invalid IPA: "+format, args...)
}

func invalidIPAWrap(cause error, format string, args ...any) *signing.Error {
	return signing.Wrap(signing.ClassSigning, signing.CodeIPAInvalid, cause, "invalid IPA: "+format, args...)
}

// AnalyzeIPA reads the bundles of an .ipa without extracting it. Errors are
// *signing.Error of ClassSigning with CodeIPAInvalid.
func AnalyzeIPA(path string) (*IPA, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, invalidIPAWrap(err, "the file is not a readable zip archive")
	}
	defer func() { _ = archive.Close() }()

	if len(archive.File) > maxZipEntries {
		return nil, invalidIPA("the archive has more than %d entries", maxZipEntries)
	}
	root, err := buildTree(archive.File)
	if err != nil {
		return nil, err
	}

	payload := root.children[payloadDirName]
	if payload == nil || !payload.dir {
		return nil, invalidIPA("the archive has no %s directory", payloadDirName)
	}
	var apps []*node
	for _, child := range payload.sortedChildren() {
		// The engine takes a directory of Payload whose extension is "app".
		if child.dir && len(child.name) > len(".app") && strings.HasSuffix(child.name, ".app") {
			apps = append(apps, child)
		}
	}
	if len(apps) != 1 {
		return nil, invalidIPA("%s must contain exactly one .app directory, found %d", payloadDirName, len(apps))
	}
	if !apps[0].hasChild(infoPlistName) {
		return nil, invalidIPA("%s has no %s", apps[0].path, infoPlistName)
	}

	main, err := readBundle(apps[0], BundleKindApp)
	if err != nil {
		return nil, err
	}
	ipa := &IPA{Main: main, Bundles: []*Bundle{main}}
	if err := ipa.collect(apps[0]); err != nil {
		return nil, err
	}
	return ipa, nil
}

// buildTree turns the archive entries into a directory tree, rejecting paths
// that could escape the extraction directory, symbolic links and duplicates.
func buildTree(files []*zip.File) (*node, error) {
	root := &node{dir: true, children: map[string]*node{}}
	for _, f := range files {
		name := f.Name
		if strings.HasPrefix(name, "/") || strings.ContainsAny(name, "\\\x00") {
			return nil, invalidIPA("unsafe entry path %q", name)
		}
		if f.Mode()&fs.ModeSymlink != 0 {
			return nil, invalidIPA("symbolic links are not supported (%q)", name)
		}
		isDir := strings.HasSuffix(name, "/") || f.Mode().IsDir()

		var parts []string
		for _, part := range strings.Split(name, "/") {
			switch part {
			case "", ".":
				continue
			case "..":
				return nil, invalidIPA("unsafe entry path %q", name)
			}
			parts = append(parts, part)
		}
		if len(parts) == 0 {
			if isDir {
				continue
			}
			return nil, invalidIPA("unsafe entry path %q", name)
		}

		current := root
		for i, part := range parts {
			last := i == len(parts)-1
			wantDir := !last || isDir
			child := current.children[part]
			switch {
			case child == nil:
				child = &node{name: part, path: joinPath(current.path, part), dir: wantDir}
				if wantDir {
					child.children = map[string]*node{}
				} else {
					child.file = f
				}
				current.children[part] = child
			case wantDir && child.dir:
				// Explicit or implicit directory seen again.
			default:
				return nil, invalidIPA("duplicate or conflicting entry %q", name)
			}
			current = child
		}
	}
	return root, nil
}

func joinPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "/" + name
}

// collect mirrors the engine's collect_embeded_bundles_from_dir: every child
// named *.dylib that is a Mach-O file is a dylib bundle; every child whose name
// has an extension and that contains an Info.plist is a bundle whose content
// is collected too unless it is an app; every other directory is walked.
func (ipa *IPA) collect(dir *node) error {
	for _, child := range dir.sortedChildren() {
		if !child.dir && strings.HasSuffix(child.name, ".dylib") {
			isMachO, err := hasMachOMagic(child.file)
			if err != nil {
				return invalidIPAWrap(err, "%s could not be read", child.path)
			}
			if isMachO {
				ipa.Bundles = append(ipa.Bundles, &Bundle{Path: child.path, Kind: BundleKindDylib})
				continue
			}
		}

		if dot := strings.LastIndexByte(child.name, '.'); dot >= 0 && child.hasChild(infoPlistName) {
			kind, known := bundleKindForExtension(child.name[dot+1:])
			if known {
				bundle, err := readBundle(child, kind)
				if err != nil {
					return err
				}
				ipa.Bundles = append(ipa.Bundles, bundle)
			}
			if kind == BundleKindApp {
				ipa.unsignedNested = append(ipa.unsignedNested, profileBundlesBelow(child)...)
			} else if err := ipa.collect(child); err != nil {
				return err
			}
			continue
		}

		if child.dir {
			if err := ipa.collect(child); err != nil {
				return err
			}
		}
	}
	return nil
}

// bundleKindForExtension mirrors BundleType::from_extension; known is false for
// the engine's Unknown type, which is never signed with a profile.
func bundleKindForExtension(ext string) (kind BundleKind, known bool) {
	switch ext {
	case "app":
		return BundleKindApp, true
	case "appex":
		return BundleKindAppExtension, true
	case "framework":
		return BundleKindFramework, true
	case "dylib":
		return BundleKindDylib, true
	}
	return "", false
}

// profileBundlesBelow lists the App and AppExtension bundles inside dir.
func profileBundlesBelow(dir *node) []string {
	var paths []string
	for _, child := range dir.sortedChildren() {
		if !child.dir {
			continue
		}
		if dot := strings.LastIndexByte(child.name, '.'); dot >= 0 && child.hasChild(infoPlistName) {
			if kind, _ := bundleKindForExtension(child.name[dot+1:]); kind.needsProfile() {
				paths = append(paths, child.path)
			}
		}
		paths = append(paths, profileBundlesBelow(child)...)
	}
	return paths
}

// readBundle reads what the checks need from one bundle directory. Only App
// and AppExtension bundles carry a profile and entitlements.
func readBundle(dir *node, kind BundleKind) (*Bundle, error) {
	bundle := &Bundle{Path: dir.path, Kind: kind}
	if !kind.needsProfile() {
		return bundle, nil
	}

	infoNode := dir.children[infoPlistName]
	if infoNode.dir {
		return nil, invalidIPA("%s/%s is a directory", dir.path, infoPlistName)
	}
	data, err := readEntry(infoNode.file, maxInfoPlistSize)
	if err != nil {
		return nil, invalidIPAWrap(err, "%s could not be read", infoNode.path)
	}
	var info infoPlist
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return nil, invalidIPAWrap(err, "%s is not a valid property list", infoNode.path)
	}
	if info.CFBundleIdentifier == "" {
		return nil, invalidIPA("%s has no CFBundleIdentifier", infoNode.path)
	}
	exe := info.CFBundleExecutable
	if exe == "" || exe == "." || exe == ".." || strings.ContainsAny(exe, "/\\\x00") {
		return nil, invalidIPA("%s has no valid CFBundleExecutable", infoNode.path)
	}
	bundle.BundleID = info.CFBundleIdentifier
	bundle.Executable = exe
	bundle.SupportedPlatforms = info.CFBundleSupportedPlatforms
	bundle.PlatformName = info.DTPlatformName
	bundle.ExtensionPoint = info.NSExtension.NSExtensionPointIdentifier

	if profileNode := dir.children[embeddedProfName]; profileNode != nil && !profileNode.dir {
		bundle.EmbeddedProfile, err = readEntry(profileNode.file, maxEmbeddedProfile)
		if err != nil {
			return nil, invalidIPAWrap(err, "%s could not be read", profileNode.path)
		}
	}

	exeNode := dir.children[exe]
	if exeNode == nil || exeNode.dir {
		return nil, invalidIPA("%s: executable %q is missing", dir.path, exe)
	}
	signature, err := readCodeSignature(exeNode.file)
	if err != nil {
		return nil, invalidIPAWrap(err, "%s: the code signature could not be read", exeNode.path)
	}
	bundle.Entitlements = signature.entitlements
	bundle.SignerCertificateSHA256 = signature.signerSHA256
	return bundle, nil
}

// readEntry reads a whole archive entry of at most limit bytes.
func readEntry(f *zip.File, limit int) ([]byte, error) {
	if f.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("entry is larger than %d bytes", limit)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(io.LimitReader(rc, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("entry is larger than %d bytes", limit)
	}
	return data, nil
}
