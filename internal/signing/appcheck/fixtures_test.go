package appcheck

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/hex"
	"io/fs"
	"math/big"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/smallstep/pkcs7"
	"howett.net/plist"

	"github.com/bitxeno/atvloadly/internal/model"
)

const (
	testTeam = "ABCDE12345"
	testUDID = "00008120-0000000000000001"
)

// testSigner is a synthetic code signing identity.
type testSigner struct {
	cert *x509.Certificate
	key  *rsa.PrivateKey
}

func (s *testSigner) sha256() string {
	sum := sha256.Sum256(s.cert.Raw)
	return hex.EncodeToString(sum[:])
}

var (
	signersOnce sync.Once
	signers     [2]*testSigner
	signersErr  error
)

// testSigners returns two distinct self-signed certificates sharing one key.
func testSigners(t *testing.T) (*testSigner, *testSigner) {
	t.Helper()
	signersOnce.Do(func() {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			signersErr = err
			return
		}
		for i := range signers {
			template := &x509.Certificate{
				SerialNumber: big.NewInt(int64(i + 1)),
				Subject:      pkix.Name{CommonName: "Synthetic Signer", OrganizationalUnit: []string{testTeam}},
				NotBefore:    time.Unix(1700000000, 0),
				NotAfter:     time.Unix(1900000000, 0),
				KeyUsage:     x509.KeyUsageDigitalSignature,
				ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
			}
			der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
			if err != nil {
				signersErr = err
				return
			}
			cert, err := x509.ParseCertificate(der)
			if err != nil {
				signersErr = err
				return
			}
			signers[i] = &testSigner{cert: cert, key: key}
		}
	})
	if signersErr != nil {
		t.Fatalf("generate signers: %v", signersErr)
	}
	return signers[0], signers[1]
}

// signature describes the code signature of a synthetic executable slice.
type signature struct {
	// entitlements is embedded as an XML entitlements blob unless nil.
	entitlements map[string]any
	// signer adds a CMS blob signed by it; adHoc adds an empty CMS blob.
	signer *testSigner
	adHoc  bool
	// none omits LC_CODE_SIGNATURE entirely.
	none bool
}

func superBlob(t *testing.T, sig signature) []byte {
	t.Helper()
	type blob struct {
		slot  uint32
		magic uint32
		data  []byte
	}
	codeDirectory := []byte("synthetic code directory")
	blobs := []blob{{slot: 0, magic: 0xfade0c02, data: codeDirectory}}
	if sig.entitlements != nil {
		xml, err := plist.MarshalIndent(sig.entitlements, plist.XMLFormat, "\t")
		if err != nil {
			t.Fatalf("marshal entitlements: %v", err)
		}
		blobs = append(blobs, blob{slot: csSlotEntitlements, magic: csMagicEmbeddedEntitlements, data: xml})
	}
	switch {
	case sig.signer != nil:
		signed, err := pkcs7.NewSignedData(codeDirectory)
		if err != nil {
			t.Fatalf("cms: %v", err)
		}
		if err := signed.AddSigner(sig.signer.cert, sig.signer.key, pkcs7.SignerInfoConfig{}); err != nil {
			t.Fatalf("cms signer: %v", err)
		}
		signed.Detach()
		cms, err := signed.Finish()
		if err != nil {
			t.Fatalf("cms finish: %v", err)
		}
		blobs = append(blobs, blob{slot: csSlotSignature, magic: csMagicBlobWrapper, data: cms})
	case sig.adHoc:
		blobs = append(blobs, blob{slot: csSlotSignature, magic: csMagicBlobWrapper})
	}

	var body bytes.Buffer
	index := make([]byte, 0, len(blobs)*8)
	offset := 12 + len(blobs)*8
	for _, b := range blobs {
		index = binary.BigEndian.AppendUint32(index, b.slot)
		index = binary.BigEndian.AppendUint32(index, uint32(offset+body.Len()))
		body.Write(binary.BigEndian.AppendUint32(nil, b.magic))
		body.Write(binary.BigEndian.AppendUint32(nil, uint32(8+len(b.data))))
		body.Write(b.data)
	}
	out := binary.BigEndian.AppendUint32(nil, csMagicEmbeddedSignature)
	out = binary.BigEndian.AppendUint32(out, uint32(offset+body.Len()))
	out = binary.BigEndian.AppendUint32(out, uint32(len(blobs)))
	out = append(out, index...)
	return append(out, body.Bytes()...)
}

// machO builds a minimal little-endian 64-bit Mach-O executable whose code
// signature starts after a stretch of padding, like __LINKEDIT data.
func machO(t *testing.T, sig signature) []byte {
	t.Helper()
	const dataOff = 0x1000
	var sigData []byte
	commands := []byte{}
	ncmds := uint32(0)
	// An unrelated load command precedes LC_CODE_SIGNATURE.
	commands = binary.LittleEndian.AppendUint32(commands, 0x2) // LC_SYMTAB
	commands = binary.LittleEndian.AppendUint32(commands, 24)
	commands = append(commands, make([]byte, 16)...)
	ncmds++
	if !sig.none {
		sigData = superBlob(t, sig)
		commands = binary.LittleEndian.AppendUint32(commands, lcCodeSignature)
		commands = binary.LittleEndian.AppendUint32(commands, 16)
		commands = binary.LittleEndian.AppendUint32(commands, dataOff)
		commands = binary.LittleEndian.AppendUint32(commands, uint32(len(sigData)))
		ncmds++
	}
	out := binary.LittleEndian.AppendUint32(nil, machMagic64)
	out = binary.LittleEndian.AppendUint32(out, 0x0100000c) // CPU_TYPE_ARM64
	out = binary.LittleEndian.AppendUint32(out, 0)
	out = binary.LittleEndian.AppendUint32(out, 2) // MH_EXECUTE
	out = binary.LittleEndian.AppendUint32(out, ncmds)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(commands)))
	out = binary.LittleEndian.AppendUint32(out, 0)
	out = binary.LittleEndian.AppendUint32(out, 0)
	out = append(out, commands...)
	if sig.none {
		return out
	}
	out = append(out, make([]byte, dataOff-len(out))...)
	return append(out, sigData...)
}

// fatMachO builds a universal binary whose architecture table lists first
// before second while first is stored after second in the file.
func fatMachO(first, second []byte, wide bool) []byte {
	const secondOff, firstOff = 0x4000, 0x8000
	magic := uint32(fatMagic)
	if wide {
		magic = fatMagic64
	}
	out := binary.BigEndian.AppendUint32(nil, magic)
	out = binary.BigEndian.AppendUint32(out, 2)
	for _, arch := range []struct {
		offset int
		data   []byte
	}{{firstOff, first}, {secondOff, second}} {
		out = binary.BigEndian.AppendUint32(out, 0x0100000c)
		out = binary.BigEndian.AppendUint32(out, 0)
		if wide {
			out = binary.BigEndian.AppendUint64(out, uint64(arch.offset))
			out = binary.BigEndian.AppendUint64(out, uint64(len(arch.data)))
			out = binary.BigEndian.AppendUint32(out, 14)
			out = binary.BigEndian.AppendUint32(out, 0)
		} else {
			out = binary.BigEndian.AppendUint32(out, uint32(arch.offset))
			out = binary.BigEndian.AppendUint32(out, uint32(len(arch.data)))
			out = binary.BigEndian.AppendUint32(out, 14)
		}
	}
	out = append(out, make([]byte, secondOff-len(out))...)
	out = append(out, second...)
	out = append(out, make([]byte, firstOff-len(out))...)
	return append(out, first...)
}

// fixtureBundle is one App or AppExtension bundle of a synthetic IPA.
type fixtureBundle struct {
	dir       string
	id        string
	platforms []string
	extPoint  string
	sig       signature
	profile   []byte
}

func (b fixtureBundle) executableName() string {
	name := path.Base(b.dir)
	return strings.TrimSuffix(name, path.Ext(name))
}

func (b fixtureBundle) files(t *testing.T) map[string][]byte {
	t.Helper()
	info := map[string]any{
		"CFBundleIdentifier": b.id,
		"CFBundleExecutable": b.executableName(),
	}
	if b.platforms != nil {
		info["CFBundleSupportedPlatforms"] = b.platforms
	}
	if b.extPoint != "" {
		info["NSExtension"] = map[string]any{"NSExtensionPointIdentifier": b.extPoint}
	}
	data, err := plist.MarshalIndent(info, plist.XMLFormat, "\t")
	if err != nil {
		t.Fatalf("marshal Info.plist: %v", err)
	}
	files := map[string][]byte{
		b.dir + "/Info.plist":                  data,
		b.dir + "/" + b.executableName():       machO(t, b.sig),
		b.dir + "/Base.lproj/Main.storyboardc": []byte("resource"),
	}
	if b.profile != nil {
		files[b.dir+"/embedded.mobileprovision"] = b.profile
	}
	return files
}

func bundleFiles(t *testing.T, bundles ...fixtureBundle) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	for _, b := range bundles {
		for name, data := range b.files(t) {
			files[name] = data
		}
	}
	return files
}

// writeIPA stores files (deflated, sorted by name) in a new .ipa file.
func writeIPA(t *testing.T, files map[string][]byte) string {
	t.Helper()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range names {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := f.Write(files[name]); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return writeFile(t, buf.Bytes())
}

func writeFile(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write ipa: %v", err)
	}
	return path
}

// writeRawZip writes entries in the given order with explicit headers.
func writeRawZip(t *testing.T, entries []rawEntry) string {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		header := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			header.SetMode(e.mode)
		}
		f, err := w.CreateHeader(header)
		if err != nil {
			t.Fatalf("zip create %s: %v", e.name, err)
		}
		if _, err := f.Write(e.data); err != nil {
			t.Fatalf("zip write %s: %v", e.name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return writeFile(t, buf.Bytes())
}

type rawEntry struct {
	name string
	data []byte
	mode fs.FileMode
}

func testProfile(bundleIDPattern string, platforms ...string) *model.MobileProvisioningProfile {
	return &model.MobileProvisioningProfile{
		ApplicationIdentifierPrefix: []string{testTeam},
		TeamIdentifier:              []string{testTeam},
		Platform:                    platforms,
		ProvisionedDevices:          []string{testUDID},
		Entitlements: map[string]any{
			"application-identifier":                 testTeam + "." + bundleIDPattern,
			"com.apple.developer.team-identifier":    testTeam,
			"get-task-allow":                         false,
			"keychain-access-groups":                 []any{testTeam + ".*", "com.apple.token"},
			"com.apple.security.application-groups":  []any{"group.example.shared", "group.example.cache"},
			"com.apple.developer.associated-domains": "*",
		},
	}
}

func plistXML(value any) ([]byte, error) {
	return plist.MarshalIndent(value, plist.XMLFormat, "\t")
}
