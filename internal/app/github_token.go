package app

import (
	"encoding/base64"
	"strings"

	"github.com/bitxeno/atvloadly/internal/signing"
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

// GetGitHubToken returns the configured plaintext token, or "" when none is set.
func GetGitHubToken() string {
	if Settings == nil {
		return ""
	}
	return strings.TrimSpace(Settings.Update.GitHubToken)
}

// GitHubTokenConfigured reports whether a token is stored.
func GitHubTokenConfigured() bool {
	return GetGitHubToken() != ""
}

func sealGitHubToken(plaintext string, sealer *signing.Sealer) (string, error) {
	blob, err := sealer.Seal([]byte(plaintext), []byte(githubTokenAAD))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(blob), nil
}

func openGitHubToken(sealed string, sealer *signing.Sealer) (string, error) {
	blob, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", err
	}
	plaintext, err := sealer.Open(blob, []byte(githubTokenAAD))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// SetGitHubToken validates and seals plaintext, staging it in memory for the
// next SaveSettings call. The caller must call SaveSettings to persist it.
func SetGitHubToken(plaintext string) error {
	t := strings.TrimSpace(plaintext)
	if err := ValidateGitHubToken(t); err != nil {
		return err
	}
	sealer, err := signing.DefaultSealer()
	if err != nil {
		return err
	}
	sealed, err := sealGitHubToken(t, sealer)
	if err != nil {
		return err
	}
	Settings.Update.GitHubToken = t
	Settings.Update.GitHubTokenSealed = sealed
	return nil
}

// ClearGitHubToken removes the token from memory and from the next persisted file.
func ClearGitHubToken() {
	if Settings == nil {
		return
	}
	Settings.Update.GitHubToken = ""
	Settings.Update.GitHubTokenSealed = ""
}

// LoadGitHubToken unseals the persisted token into memory. Call it once after
// signing.Configure so the deployment key is available. A missing token is
// not an error; a corrupt one clears the in-memory copy and returns the error.
func LoadGitHubToken() error {
	if Settings == nil {
		return nil
	}
	sealed := strings.TrimSpace(Settings.Update.GitHubTokenSealed)
	if sealed == "" {
		Settings.Update.GitHubToken = ""
		return nil
	}
	sealer, err := signing.DefaultSealer()
	if err != nil {
		Settings.Update.GitHubToken = ""
		return err
	}
	plaintext, err := openGitHubToken(sealed, sealer)
	if err != nil {
		Settings.Update.GitHubToken = ""
		return err
	}
	Settings.Update.GitHubToken = strings.TrimSpace(plaintext)
	return nil
}
