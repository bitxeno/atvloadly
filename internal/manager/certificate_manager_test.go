package manager

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pkcs12 "software.sslmate.com/src/go-pkcs12"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/signing"
)

// testP12 builds a self-signed code signing identity. password == "" produces
// the shape SideStore exports: a plain key bag without a MAC.
func testP12(t *testing.T, password string) ([]byte, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Synthetic Identity", OrganizationalUnit: []string{"ABCDE12345"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	var data []byte
	if password == "" {
		data, err = pkcs12.Passwordless.Encode(key, cert, nil, "")
	} else {
		data, err = pkcs12.LegacyDES.Encode(key, cert, nil, password)
	}
	if err != nil {
		t.Fatal(err)
	}
	return data, key
}

// fakeEngine puts a plumesign on PATH that records its arguments, copies the
// file given to -i and succeeds. It returns the recorded argument path and the
// captured input path. The copy is taken while the engine runs, because the
// caller removes the input as soon as it returns.
func fakeEngine(t *testing.T) (record, captured string) {
	t.Helper()
	dir := t.TempDir()
	record = filepath.Join(dir, "args")
	captured = filepath.Join(dir, "input.p12")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + record + "\n" +
		"prev=''\n" +
		"for a in \"$@\"; do\n" +
		"  if [ \"$prev\" = '-i' ]; then cp \"$a\" " + captured + "; fi\n" +
		"  prev=\"$a\"\n" +
		"done\n"
	if err := os.WriteFile(filepath.Join(dir, "plumesign"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return record, captured
}

// setupImport prepares the configuration the import path reads.
func setupImport(t *testing.T) {
	t.Helper()
	previousConfig, previousSettings := app.Config, app.Settings
	app.Config = &app.Configuration{}
	app.Config.Server.DataDir = t.TempDir()
	app.Settings = &app.SettingsConfiguration{}
	t.Cleanup(func() { app.Config, app.Settings = previousConfig, previousSettings })
}

// recordedArgs reads the argument file the fake engine wrote.
func recordedArgs(t *testing.T, record string) []string {
	t.Helper()
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("the signing engine was not run: %v", err)
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

// argValue returns the value following flag in args.
func argValue(t *testing.T, args []string, flag string) string {
	t.Helper()
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	t.Fatalf("flag %s is missing from %q", flag, args)
	return ""
}

// A SideStore export reaches the engine as a file it can read, protected by a
// throwaway password rather than by the (empty) password of the user.
func TestImportCertificateConvertsSideStoreExport(t *testing.T) {
	setupImport(t)
	record, captured := fakeEngine(t)

	p12, _ := testP12(t, "")
	if err := ImportCertificate("user@example.com", "", p12); err != nil {
		t.Fatalf("ImportCertificate: %v", err)
	}

	args := recordedArgs(t, record)
	if got := argValue(t, args, "-u"); got != "user@example.com" {
		t.Fatalf("-u = %q, want the account", got)
	}
	password := argValue(t, args, "-p")
	if password == "" {
		t.Fatal("the engine was handed an empty password")
	}
	path := argValue(t, args, "-i")

	// The engine must receive a shrouded, MAC-protected file it can open with
	// that password; the raw SideStore export is not that file.
	handed, err := os.ReadFile(captured)
	if err != nil {
		t.Fatalf("the engine read no file: %v", err)
	}
	if string(handed) == string(p12) {
		t.Fatal("the raw upload was handed to the engine")
	}
	if _, err := signing.DecodeP12(handed, password); err != nil {
		t.Fatalf("the engine cannot read the file it was handed: %v", err)
	}
	if _, err := signing.DecodeP12(p12, ""); err != nil {
		t.Fatalf("the SideStore shape must stay readable in process: %v", err)
	}

	// The file is removed once the engine returns.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("the temporary file %s was not removed", path)
	}
}

// The password of the user never reaches the command line.
func TestImportCertificateHidesTheUserPassword(t *testing.T) {
	setupImport(t)
	record, _ := fakeEngine(t)

	const userPassword = "S3cret-Pässwörd"
	p12, _ := testP12(t, userPassword)
	if err := ImportCertificate("user@example.com", userPassword, p12); err != nil {
		t.Fatalf("ImportCertificate: %v", err)
	}

	args := recordedArgs(t, record)
	for _, arg := range args {
		if arg == userPassword {
			t.Fatalf("the password of the user reached the command line: %q", args)
		}
	}
	if password := argValue(t, args, "-p"); password == userPassword {
		t.Fatal("the engine was handed the password of the user")
	}
}

// An unusable upload is refused with a stable signing code and the engine is
// never run.
func TestImportCertificateRefusesInvalidP12(t *testing.T) {
	setupImport(t)
	record, _ := fakeEngine(t)

	err := ImportCertificate("user@example.com", "", []byte("not a PKCS#12 file"))
	if got := signing.CodeOf(err); got != signing.CodeP12Invalid {
		t.Fatalf("code = %q, want %q (err %v)", got, signing.CodeP12Invalid, err)
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("the signing engine ran for an invalid file")
	}
}
