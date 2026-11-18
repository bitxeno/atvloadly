package manager

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	execx "github.com/bitxeno/atvloadly/internal/exec"
	"github.com/bitxeno/atvloadly/internal/model"
)

func TestEngineTimeoutAfterUpload(t *testing.T) {
	dir := t.TempDir()
	// Emit the same progress messages as plumesign, then wait for the deadline.
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\"\nexec sleep 30\n"
	if err := os.WriteFile(filepath.Join(dir, "plumesign"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, tc := range []struct {
		name    string
		lines   []string
		pending bool
		busy    bool
	}{
		{"waiting for confirmation", []string{"AFC upload progress: 100%"}, true, false},
		{"device reports busy extension", []string{"AFC upload progress: 100%", "PlugInKit Code=14: plugins are busy"}, true, true},
		{"still transferring", []string{"AFC upload progress: 95%"}, false, false},
		{"already installed", []string{"AFC upload progress: 100%", "Installation complete!"}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mgr := NewInstallManager()
			defer mgr.Close()
			// External signing retains the previous engine attempt's log.
			mgr.WriteLog("AFC upload progress: 100%\n")
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			err := mgr.runEngine(ctx, tc.lines, dir, nil, nil)
			if !errors.Is(err, execx.ErrCommandTimeout) {
				t.Fatalf("expected a command timeout, got %v", err)
			}
			if got := strings.Contains(err.Error(), "waiting for the device"); got != tc.pending {
				t.Fatalf("pending installation = %t, got %v", tc.pending, err)
			}
			if got := strings.Contains(err.Error(), "busy app extension"); got != tc.busy {
				t.Fatalf("busy extension = %t, got %v", tc.busy, err)
			}
			if retry := shouldRetryInstall(model.SigningModeAppleID, err, nil); retry == tc.pending {
				t.Fatalf("retry = %t after %v", retry, err)
			}
			if tc.pending {
				external := classifyExternalFailure(engineMarkerInstalling, err)
				if shouldRetryInstall(model.SigningModeExternalCertificate, external, nil) {
					t.Fatalf("unexpected retry of external installation: %v", external)
				}
			}
		})
	}
}

func TestBuildInstallArgsUsesUDIDForRSD(t *testing.T) {
	args := buildInstallArgs(InstallOptions{
		UDID:    "test-device-udid",
		IP:      "192.0.2.10",
		Port:    49152,
		Account: "user@example.com",
		IpaPath: "app.ipa",
	}, "embedded.mobileprovision")

	if args[0] != "sign-rsd" {
		t.Fatalf("expected sign-rsd command, got %q", args[0])
	}
	udidFlag := indexArg(args, "--udid")
	if udidFlag == -1 || udidFlag+1 >= len(args) {
		t.Fatal("expected --udid argument")
	}
	if args[udidFlag+1] != "test-device-udid" {
		t.Fatalf("UDID = %q, want %q", args[udidFlag+1], "test-device-udid")
	}
}

func TestBuildInstallArgsAppleID(t *testing.T) {
	tests := []struct {
		name string
		opts InstallOptions
		want []string
	}{
		{
			name: "lockdown",
			opts: InstallOptions{UDID: "DEVICE", Account: "user@example.com", IpaPath: "app.ipa"},
			want: []string{"sign", "--apple-id", "--register-and-install", "--output-provision", "embedded.mobileprovision", "--udid", "DEVICE", "-u", "user@example.com", "-p", "app.ipa"},
		},
		{
			name: "rsd with custom name",
			opts: InstallOptions{UDID: "DEVICE", IP: "192.0.2.10", Port: 49152, Account: "user@example.com", IpaPath: "app.ipa", CustomName: "My App", SigningMode: model.SigningModeAppleID},
			want: []string{"sign-rsd", "--apple-id", "--register-and-install", "--output-provision", "embedded.mobileprovision", "--ip", "192.0.2.10", "--port", "49152", "--udid", "DEVICE", "-u", "user@example.com", "-p", "app.ipa", "--custom-name", "My App"},
		},
		{
			name: "lockdown with custom identifier",
			opts: InstallOptions{UDID: "DEVICE", Account: "user@example.com", IpaPath: "app.ipa", CustomIdentifier: "app.custom"},
			want: []string{"sign", "--apple-id", "--register-and-install", "--output-provision", "embedded.mobileprovision", "--udid", "DEVICE", "-u", "user@example.com", "-p", "app.ipa", "--custom-identifier=app.custom"},
		},
		{
			// A refresh re-signs the stored IPA, which still carries the bundle
			// identifier of the original package: the custom identifier must be
			// passed again or the app would be refreshed under another App ID.
			// --refresh itself is appended by startAppleID, not here.
			name: "refresh with custom identifier",
			opts: InstallOptions{UDID: "DEVICE", Account: "user@example.com", IpaPath: "app.ipa", CustomIdentifier: "app.custom", RefreshMode: true},
			want: []string{"sign", "--apple-id", "--register-and-install", "--output-provision", "embedded.mobileprovision", "--udid", "DEVICE", "-u", "user@example.com", "-p", "app.ipa", "--custom-identifier=app.custom"},
		},
		{
			// clap reads a separate value starting with '-' as a flag, so the
			// value is always attached to its flag.
			name: "custom identifier with a leading hyphen",
			opts: InstallOptions{UDID: "DEVICE", Account: "user@example.com", IpaPath: "app.ipa", CustomIdentifier: "-beta.example.app"},
			want: []string{"sign", "--apple-id", "--register-and-install", "--output-provision", "embedded.mobileprovision", "--udid", "DEVICE", "-u", "user@example.com", "-p", "app.ipa", "--custom-identifier=-beta.example.app"},
		},
		{
			// The custom icon is an image path: it is attached to its flag
			// like the custom identifier so a leading hyphen is never read
			// as another flag.
			name: "custom icon",
			opts: InstallOptions{UDID: "DEVICE", Account: "user@example.com", IpaPath: "app.ipa", CustomIconPath: "/data/ipa/1/custom-icon.png"},
			want: []string{"sign", "--apple-id", "--register-and-install", "--output-provision", "embedded.mobileprovision", "--udid", "DEVICE", "-u", "user@example.com", "-p", "app.ipa", "--custom-icon=/data/ipa/1/custom-icon.png"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildInstallArgs(tt.opts, "embedded.mobileprovision"); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("args =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func indexArg(args []string, want string) int {
	for i, arg := range args {
		if arg == want {
			return i
		}
	}
	return -1
}
