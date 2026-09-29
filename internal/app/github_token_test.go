package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitxeno/atvloadly/internal/signing"
	"github.com/bitxeno/atvloadly/internal/utils"
)

func TestValidateGitHubToken(t *testing.T) {
	if err := ValidateGitHubToken("ghp_1234567890abcdef1234567890ab"); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	for _, bad := range []string{"", "   ", "short", "ghp_****abcd", "has space inside token value", "tab\there"} {
		if err := ValidateGitHubToken(bad); err == nil {
			t.Fatalf("invalid token %q accepted", bad)
		}
	}
}

func configureTestSealer(t *testing.T) {
	t.Helper()
	keyFile := filepath.Join(t.TempDir(), "keys", "signing-identity.key")
	signing.Configure(keyFile, t.TempDir())
	t.Cleanup(func() { signing.Configure("", "") })
}

func TestSetAndLoadGitHubTokenRoundTrip(t *testing.T) {
	old := Settings
	t.Cleanup(func() { Settings = old })
	Settings = &SettingsConfiguration{}

	configureTestSealer(t)

	plaintext := "ghp_1234567890abcdef1234567890ab"
	if err := SetGitHubToken(plaintext); err != nil {
		t.Fatalf("SetGitHubToken: %v", err)
	}
	if Settings.Update.GitHubTokenSealed == "" {
		t.Fatal("sealed token not staged")
	}
	if strings.Contains(Settings.Update.GitHubTokenSealed, plaintext) {
		t.Fatal("sealed blob contains the plaintext token")
	}
	if GetGitHubToken() != plaintext {
		t.Fatalf("GetGitHubToken = %q, want %q", GetGitHubToken(), plaintext)
	}
	if !GitHubTokenConfigured() {
		t.Fatal("token not reported as configured after setting it")
	}

	// Simulate a restart: drop the in-memory copy and unseal from disk state.
	Settings.Update.GitHubToken = ""
	if err := LoadGitHubToken(); err != nil {
		t.Fatalf("LoadGitHubToken: %v", err)
	}
	if GetGitHubToken() != plaintext {
		t.Fatalf("after reload token = %q, want %q", GetGitHubToken(), plaintext)
	}

	ClearGitHubToken()
	if GitHubTokenConfigured() || Settings.Update.GitHubTokenSealed != "" {
		t.Fatal("ClearGitHubToken did not remove the token")
	}
}

func TestSealedTokenNeverSerializedAsPlaintext(t *testing.T) {
	old := Settings
	t.Cleanup(func() { Settings = old })
	Settings = &SettingsConfiguration{}

	configureTestSealer(t)

	plaintext := "github_pat_1234567890abcdefghij1234567890abcd"
	if err := SetGitHubToken(plaintext); err != nil {
		t.Fatalf("SetGitHubToken: %v", err)
	}
	data := string(utils.ToIndentJSON(Settings))
	if strings.Contains(data, plaintext) {
		t.Fatal("settings JSON contains the plaintext token")
	}
	if !strings.Contains(data, "github_token_sealed") {
		t.Fatal("settings JSON does not persist the sealed token")
	}
}
