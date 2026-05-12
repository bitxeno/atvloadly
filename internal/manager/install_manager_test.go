package manager

import (
	"path/filepath"
	"testing"
)

func TestBuildInstallArgsUsesPairingFileForRSD(t *testing.T) {
	t.Setenv("HOME", "/tmp/atvloadly-test-home")

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
	if containsArg(args, "--udid") {
		t.Fatal("sign-rsd does not accept --udid")
	}

	pairingFlag := indexArg(args, "--pairing-file")
	if pairingFlag == -1 || pairingFlag+1 >= len(args) {
		t.Fatal("expected --pairing-file argument")
	}

	wantPairingFile := filepath.Join("/tmp/atvloadly-test-home", ".config/PlumeImpactor/pairing_files/test-device-udid.plist")
	if args[pairingFlag+1] != wantPairingFile {
		t.Fatalf("pairing file = %q, want %q", args[pairingFlag+1], wantPairingFile)
	}
}

func containsArg(args []string, want string) bool {
	return indexArg(args, want) != -1
}

func indexArg(args []string, want string) int {
	for i, arg := range args {
		if arg == want {
			return i
		}
	}
	return -1
}
