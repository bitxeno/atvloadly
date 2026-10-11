package manager

import (
	"reflect"
	"testing"

	"github.com/bitxeno/atvloadly/internal/model"
)

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
