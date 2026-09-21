package manager

import (
	"testing"
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

func indexArg(args []string, want string) int {
	for i, arg := range args {
		if arg == want {
			return i
		}
	}
	return -1
}
