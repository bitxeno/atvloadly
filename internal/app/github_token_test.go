package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitxeno/atvloadly/internal/secret"
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

func configureTestSecret(t *testing.T) {
	t.Helper()
	keyFile := filepath.Join(t.TempDir(), "keys", "secret-store.key")
	secret.Configure(keyFile)
	t.Cleanup(func() { secret.Configure("") })
}

func TestSetAndGetGitHubTokenRoundTrip(t *testing.T) {
	old := Settings
	t.Cleanup(func() { Settings = old })
	Settings = &SettingsConfiguration{}

	configureTestSecret(t)

	plaintext := "ghp_1234567890abcdef1234567890ab"
	if err := SetGitHubToken(plaintext); err != nil {
		t.Fatalf("SetGitHubToken: %v", err)
	}
	// Settings holds the sealed blob only.
	if Settings.Update.GitHubToken == "" {
		t.Fatal("sealed token not staged")
	}
	if strings.Contains(Settings.Update.GitHubToken, plaintext) {
		t.Fatal("sealed blob contains the plaintext token")
	}
	if GetGitHubToken() != plaintext {
		t.Fatalf("GetGitHubToken = %q, want %q", GetGitHubToken(), plaintext)
	}
	if !GitHubTokenConfigured() {
		t.Fatal("token not reported as configured after setting it")
	}
	if err := CheckGitHubToken(); err != nil {
		t.Fatalf("CheckGitHubToken: %v", err)
	}

	DeleteGitHubToken()
	if GitHubTokenConfigured() || Settings.Update.GitHubToken != "" {
		t.Fatal("DeleteGitHubToken did not remove the token")
	}
	if GetGitHubToken() != "" {
		t.Fatal("GetGitHubToken returns a value after delete")
	}
}

func TestGetGitHubTokenRejectsTamperedValue(t *testing.T) {
	old := Settings
	t.Cleanup(func() { Settings = old })
	Settings = &SettingsConfiguration{}

	configureTestSecret(t)

	plaintext := "ghp_1234567890abcdef1234567890ab"
	if err := SetGitHubToken(plaintext); err != nil {
		t.Fatalf("SetGitHubToken: %v", err)
	}
	// Tamper with the sealed blob: reads must fail closed.
	Settings.Update.GitHubToken += "x"
	if GetGitHubToken() != "" {
		t.Fatal("GetGitHubToken returns a value for a tampered blob")
	}
	if err := CheckGitHubToken(); err == nil {
		t.Fatal("CheckGitHubToken accepts a tampered blob")
	}
}

func TestSealedTokenNeverSerializedAsPlaintext(t *testing.T) {
	old := Settings
	t.Cleanup(func() { Settings = old })
	Settings = &SettingsConfiguration{}

	configureTestSecret(t)

	plaintext := "github_pat_1234567890abcdefghij1234567890abcd"
	if err := SetGitHubToken(plaintext); err != nil {
		t.Fatalf("SetGitHubToken: %v", err)
	}
	data := string(utils.ToIndentJSON(Settings))
	if strings.Contains(data, plaintext) {
		t.Fatal("settings JSON contains the plaintext token")
	}
	if !strings.Contains(data, "github_token") {
		t.Fatal("settings JSON does not persist the sealed token")
	}
}
