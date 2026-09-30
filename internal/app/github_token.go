package app

import (
	"strings"

	"github.com/bitxeno/atvloadly/internal/secret"
	"github.com/go-errors/errors"
)

// githubTokenAAD binds the sealed GitHub token to its purpose so a sealed
// blob cannot be transplanted into another field.
const githubTokenAAD = "atvloadly-github-token:v1"

// ValidateGitHubToken rejects empty-surrounding whitespace, control
// characters and masked previews so a copied mask can never be stored.
func ValidateGitHubToken(token string) error {
	t := strings.TrimSpace(token)
	if t == "" {
		return errors.New("github token is empty")
	}
	if strings.Contains(t, "*") {
		return errors.New("please enter the full token instead of its masked preview")
	}
	if len(t) < 8 || len(t) > 512 {
		return errors.New("github token length must be between 8 and 512 characters")
	}
	for _, r := range t {
		if r <= 0x20 || r == 0x7f {
			return errors.New("github token must not contain whitespace")
		}
		if r > 0x7e {
			return errors.New("github token must be printable ASCII")
		}
	}
	return nil
}

// sealedGitHubToken returns the sealed blob stored in settings.
func sealedGitHubToken() string {
	if Settings == nil {
		return ""
	}
	return strings.TrimSpace(Settings.Update.GitHubToken)
}

// GetGitHubToken returns the configured plaintext token, or "" when none is
// set or the sealed value cannot be opened.
func GetGitHubToken() string {
	sealed := sealedGitHubToken()
	if sealed == "" {
		return ""
	}
	store, err := secret.Default()
	if err != nil {
		return ""
	}
	plaintext, err := store.OpenString(sealed, githubTokenAAD)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(plaintext)
}

// GitHubTokenConfigured reports whether a token is stored. It checks the
// sealed value only and never decrypts.
func GitHubTokenConfigured() bool {
	return sealedGitHubToken() != ""
}

// SetGitHubToken validates and seals plaintext, staging it for the next
// SaveSettings call. The caller must call SaveSettings to persist it.
func SetGitHubToken(plaintext string) error {
	t := strings.TrimSpace(plaintext)
	if err := ValidateGitHubToken(t); err != nil {
		return err
	}
	store, err := secret.Default()
	if err != nil {
		return err
	}
	sealed, err := store.SealString(t, githubTokenAAD)
	if err != nil {
		return err
	}
	Settings.Update.GitHubToken = sealed
	return nil
}

// DeleteGitHubToken removes the token from the next persisted file.
func DeleteGitHubToken() {
	if Settings == nil {
		return
	}
	Settings.Update.GitHubToken = ""
}

// CheckGitHubToken reports whether the persisted token can be opened. A
// missing token is not an error.
func CheckGitHubToken() error {
	sealed := sealedGitHubToken()
	if sealed == "" {
		return nil
	}
	store, err := secret.Default()
	if err != nil {
		return err
	}
	_, err = store.OpenString(sealed, githubTokenAAD)
	return err
}
