package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/signing"
	"github.com/gookit/event"
)

func testExternalSigning() *ExternalSigning {
	return &ExternalSigning{
		CertificatePath: "/ws/cert.pem",
		PrivateKeyPath:  "/ws/key.pem",
		ProfilePath:     "/ws/profile.mobileprovision",
		OutputPath:      "/ws/signed.ipa",
		PairingFile:     "/pairing/DEVICE.plist",
		HomeDir:         "/ws/home",
		TempDir:         "/ws/tmp",
	}
}

func TestBuildExternalInstallArgs(t *testing.T) {
	withOptions := testExternalSigning()
	withOptions.CustomIdentifier = "app.example.signed"
	withHyphenValues := testExternalSigning()
	withHyphenValues.CustomIdentifier = "-beta.example.app"

	tests := []struct {
		name string
		opts InstallOptions
		want []string
	}{
		{
			name: "lockdown",
			opts: InstallOptions{UDID: "DEVICE", Account: "user@example.com", IpaPath: "/data/tmp/app.ipa", RefreshMode: true, External: testExternalSigning()},
			want: []string{"sign", "--package", "/data/tmp/app.ipa", "--pem", "/ws/cert.pem", "/ws/key.pem", "--provision", "/ws/profile.mobileprovision", "--register-and-install", "--udid", "DEVICE", "--output", "/ws/signed.ipa"},
		},
		{
			name: "rsd",
			opts: InstallOptions{UDID: "DEVICE", IP: "192.0.2.10", Port: 49152, IpaPath: "/data/tmp/app.ipa", External: testExternalSigning()},
			want: []string{"sign-rsd", "--package", "/data/tmp/app.ipa", "--pem", "/ws/cert.pem", "/ws/key.pem", "--provision", "/ws/profile.mobileprovision", "--register-and-install", "--ip", "192.0.2.10", "--port", "49152", "--pairing-file", "/pairing/DEVICE.plist", "--udid", "DEVICE", "--output", "/ws/signed.ipa"},
		},
		{
			name: "optional flags",
			opts: InstallOptions{UDID: "DEVICE", IpaPath: "/data/tmp/app.ipa", CustomName: "My App", RemoveExtensions: true, External: withOptions},
			want: []string{"sign", "--package", "/data/tmp/app.ipa", "--pem", "/ws/cert.pem", "/ws/key.pem", "--provision", "/ws/profile.mobileprovision", "--register-and-install", "--udid", "DEVICE", "--output", "/ws/signed.ipa", "--custom-identifier=app.example.signed", "--remove-extensions", "--custom-name=My App"},
		},
		{
			name: "values starting with a hyphen",
			opts: InstallOptions{UDID: "DEVICE", IP: "192.0.2.10", Port: 49152, IpaPath: "/data/tmp/app.ipa", CustomName: "-My App", External: withHyphenValues},
			want: []string{"sign-rsd", "--package", "/data/tmp/app.ipa", "--pem", "/ws/cert.pem", "/ws/key.pem", "--provision", "/ws/profile.mobileprovision", "--register-and-install", "--ip", "192.0.2.10", "--port", "49152", "--pairing-file", "/pairing/DEVICE.plist", "--udid", "DEVICE", "--output", "/ws/signed.ipa", "--custom-identifier=-beta.example.app", "--custom-name=-My App"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildExternalInstallArgs(tt.opts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("args =\n%q\nwant\n%q", got, tt.want)
			}
			for _, forbidden := range []string{"--apple-id", "-u", "--username", "--refresh", "user@example.com"} {
				if indexArg(got, forbidden) != -1 {
					t.Fatalf("external args contain %q: %q", forbidden, got)
				}
			}
		})
	}
}

func TestExternalRunEnvIsolatesHomeAndTempDir(t *testing.T) {
	base := []string{"HOME=/root", "TMPDIR=/tmp", "HTTPS_PROXY=http://proxy:3128", "PATH=/usr/bin"}
	env := externalRunEnv(base, *testExternalSigning())

	values := map[string][]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		values[k] = append(values[k], v)
	}
	want := map[string]string{"HOME": "/ws/home", "TMPDIR": "/ws/tmp", "HTTPS_PROXY": "http://proxy:3128", "PATH": "/usr/bin"}
	for k, v := range want {
		if got := values[k]; len(got) != 1 || got[0] != v {
			t.Errorf("%s = %q, want exactly [%q]", k, got, v)
		}
	}
}

func TestClassifyExternalFailure(t *testing.T) {
	const logPrefix = "[2026-09-25 13:16:28 INFO  plumesign::commands::sign] "
	exitErr := errors.New("exit status 1")

	tests := []struct {
		name   string
		output string
		runErr error
		class  signing.Class
		code   string
	}{
		{
			name:   "usbmuxd device unavailable before signing",
			output: logPrefix + "Detected app type: Default\nError: Idevice error: device socket io failed\n\nCaused by:\n    0: device socket io failed\n    1: No such file or directory (os error 2)\n",
			runErr: exitErr,
			class:  signing.ClassTransport,
			code:   signing.CodeTransportFailed,
		},
		{
			name:   "device not listed by usbmuxd",
			output: "Error: Other error: Device ID 00008120-0000000000000001 not found\n",
			runErr: exitErr,
			class:  signing.ClassTransport,
			code:   signing.CodeTransportFailed,
		},
		{
			name:   "rsd pairing record missing a field",
			output: "Error: io on plist\n\nCaused by:\n    Serde(\"missing field `public_key`\")\n",
			runErr: exitErr,
			class:  signing.ClassTransport,
			code:   signing.CodePairingRecordInvalid,
		},
		{
			name:   "rsd pairing record not a dictionary",
			output: logPrefix + "Detected app type: Default\nError: io on plist\n\nCaused by:\n    Serde(\"invalid type: string \\\"pairing\\\", expected a map\")\n",
			runErr: exitErr,
			class:  signing.ClassTransport,
			code:   signing.CodePairingRecordInvalid,
		},
		{
			name:   "rsd connection refused",
			output: "Error: Connection refused (os error 111)\n",
			runErr: exitErr,
			class:  signing.ClassTransport,
			code:   signing.CodeTransportFailed,
		},
		{
			name:   "timeout without output",
			output: "",
			runErr: errors.New("installation exceeded 60-minute timeout limit"),
			class:  signing.ClassTransport,
			code:   signing.CodeTransportFailed,
		},
		{
			name:   "private key file unreadable",
			output: logPrefix + "Detected app type: Default\nError: I/O error: No such file or directory (os error 2)\n\nCaused by:\n    No such file or directory (os error 2)\n",
			runErr: exitErr,
			class:  signing.ClassSigning,
			code:   signing.CodeEngineFailed,
		},
		{
			name:   "provisioning profile rejected",
			output: logPrefix + "Detected app type: Default\nError: Entitlements not found\n",
			runErr: exitErr,
			class:  signing.ClassSigning,
			code:   signing.CodeEngineFailed,
		},
		{
			name:   "certificate pem rejected",
			output: "Error: Certificate PEM error: malformed\n",
			runErr: exitErr,
			class:  signing.ClassSigning,
			code:   signing.CodeEngineFailed,
		},
		{
			name:   "platform refused after rsd connection",
			output: logPrefix + "Product Type: AppleTV14,1\nError: iOS bundle can't install to target device type: AppleTV\n",
			runErr: exitErr,
			class:  signing.ClassSigning,
			code:   signing.CodeEngineFailed,
		},
		{
			name:   "failure while signing",
			output: logPrefix + "Signing bundle: /tmp/plume_stage/Payload/App.app\nError: Idevice error: device socket io failed\n",
			runErr: exitErr,
			class:  signing.ClassSigning,
			code:   signing.CodeEngineFailed,
		},
		{
			name:   "failure while installing",
			output: logPrefix + "Signing bundle: /tmp/plume_stage/Payload/App.app\n" + logPrefix + "Installing to device: iPhone\n" + logPrefix + "Installation progress: 40%\nError: Codesign error: ApplicationVerificationFailed\n",
			runErr: exitErr,
			class:  signing.ClassTransport,
			code:   signing.CodeTransportFailed,
		},
		{
			name:   "failure after installation",
			output: logPrefix + "Signing bundle: x\n" + logPrefix + "Installing to device: iPhone\n" + logPrefix + "Installation complete!\nError: I/O error: No space left on device (os error 28)\n",
			runErr: exitErr,
			class:  signing.ClassSigning,
			code:   signing.CodeSignedOutputMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := classifyExternalFailure(tt.output, tt.runErr)
			if err.Class != tt.class || err.Code != tt.code {
				t.Fatalf("classified as %s/%s, want %s/%s (%v)", err.Class, err.Code, tt.class, tt.code, err)
			}
			if !errors.Is(err, tt.runErr) {
				t.Fatalf("classified error does not wrap the run error: %v", err)
			}
		})
	}
}

func TestShouldRetryInstall(t *testing.T) {
	transport := signing.Errorf(signing.ClassTransport, signing.CodeTransportFailed, "device lost")
	tests := []struct {
		name   string
		mode   model.SigningMode
		err    error
		ctxErr error
		want   bool
	}{
		{name: "historical apple id", mode: "", err: errors.New("exit status 1"), want: true},
		{name: "apple id", mode: model.SigningModeAppleID, err: errors.New("exit status 1"), want: true},
		{name: "apple id cancelled", mode: model.SigningModeAppleID, err: errors.New("exit status 1"), ctxErr: context.Canceled, want: true},
		{name: "external transport", mode: model.SigningModeExternalCertificate, err: transport, want: true},
		{name: "external device unreachable", mode: model.SigningModeExternalCertificate, err: signing.Errorf(signing.ClassTransport, signing.CodeDeviceUnreachable, "afc"), want: true},
		{name: "external wrapped transport", mode: model.SigningModeExternalCertificate, err: errors.Join(errors.New("attempt"), transport), want: true},
		{name: "external transport cancelled", mode: model.SigningModeExternalCertificate, err: transport, ctxErr: context.Canceled, want: false},
		{name: "external pairing record invalid", mode: model.SigningModeExternalCertificate, err: signing.Errorf(signing.ClassTransport, signing.CodePairingRecordInvalid, "pairing"), want: false},
		{name: "external signing", mode: model.SigningModeExternalCertificate, err: signing.Errorf(signing.ClassSigning, signing.CodeEngineFailed, "sign"), want: false},
		{name: "external identity", mode: model.SigningModeExternalCertificate, err: signing.Errorf(signing.ClassIdentity, signing.CodeKeyMismatch, "key"), want: false},
		{name: "external unclassified", mode: model.SigningModeExternalCertificate, err: errors.New("exit status 1"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRetryInstall(tt.mode, tt.err, tt.ctxErr); got != tt.want {
				t.Fatalf("shouldRetryInstall = %v, want %v", got, tt.want)
			}
		})
	}
}

// An RSD installation whose remote pairing record is missing or unreadable
// fails before the engine runs, telling the user to pair the device again.
func TestStartExternalRefusesInvalidPairingRecord(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name    string
		missing bool
		content string
	}{
		{name: "missing", missing: true},
		{name: "empty", content: ""},
		{name: "not a plist", content: "\x00\x01 not a pairing record"},
		{name: "string instead of a dictionary", content: `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><string>pairing</string></plist>`},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := testExternalSigning()
			ext.PairingFile = filepath.Join(dir, fmt.Sprintf("record-%d.plist", i))
			if !tt.missing {
				if err := os.WriteFile(ext.PairingFile, []byte(tt.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			mgr := NewInstallManager()
			defer mgr.Close()

			err := mgr.startExternal(context.Background(), InstallOptions{
				UDID:        "00008120-0000000000000001",
				IP:          "192.0.2.10",
				Port:        49152,
				IpaPath:     "/data/tmp/app.ipa",
				SigningMode: model.SigningModeExternalCertificate,
				External:    ext,
			})
			if signing.ClassOf(err) != signing.ClassTransport || signing.CodeOf(err) != signing.CodePairingRecordInvalid {
				t.Fatalf("err = %v (class %q code %q), want transport/%s", err, signing.ClassOf(err), signing.CodeOf(err), signing.CodePairingRecordInvalid)
			}
			if output := mgr.OutputLog(); output != "" {
				t.Fatalf("the engine ran: %q", output)
			}
		})
	}
}

func TestCheckPairingRecordAcceptsDictionary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "record.plist")
	record := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>HostIdentifier</key><string>00000000-0000-0000-0000-000000000001</string></dict></plist>`
	if err := os.WriteFile(path, []byte(record), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkPairingRecord(path); err != nil {
		t.Fatalf("checkPairingRecord = %v, want nil for a plist dictionary", err)
	}
}

func TestOutputWriterKeepsRecentOutput(t *testing.T) {
	w := newOutputWriter(event.NewManager("test"))
	w.limit = 8

	_, _ = w.Write([]byte("0123456789"))
	mark := w.Mark()
	_, _ = w.Write([]byte("abcdef"))

	got := w.String()
	if !strings.HasPrefix(got, outputTruncatedMarker) {
		t.Fatalf("truncated output lacks the marker: %q", got)
	}
	if retained := strings.TrimPrefix(got, outputTruncatedMarker); len(retained) > w.limit+w.limit/4 || !strings.HasSuffix(retained, "89abcdef") {
		t.Fatalf("retained output = %q, want the most recent bytes within the limit", retained)
	}
	if since := w.Since(mark); since != "abcdef" {
		t.Fatalf("Since(mark) = %q, want %q", since, "abcdef")
	}

	w.Reset()
	_, _ = w.Write([]byte("new"))
	if got := w.String(); got != "new" {
		t.Fatalf("output after reset = %q, want %q", got, "new")
	}
}
