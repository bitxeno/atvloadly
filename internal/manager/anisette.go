package manager

import (
	"errors"
	"net/url"
	"strings"

	"github.com/bitxeno/atvloadly/internal/exec"
	"github.com/bitxeno/atvloadly/internal/log"
)

// ResetAnisette drops the Anisette provisioning data of a single account
// through the engine, so a new device identity is generated the next time that
// account is used. Anisette state is provisioned per account, so the engine
// needs the account to know which state to discard. The command is local, so
// it needs no working directory or proxy environment.
func ResetAnisette(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return errors.New("account email is required")
	}

	if _, err := exec.NewCommand("plumesign", "anisette", "reset", "-u", email).CombinedOutput(); err != nil {
		log.Err(err).Msgf("Error resetting anisette data for %s", email)
		return err
	}
	log.Infof("Reset anisette data for %s", email)
	return nil
}

// AnisetteServerState is the anisette server the engine provisions against,
// with whether it is the engine default rather than a configured one.
type AnisetteServerState struct {
	Server    string `json:"server"`
	IsDefault bool   `json:"is_default"`
}

// AnisetteServer returns the anisette server the engine provisions against.
func AnisetteServer() (*AnisetteServerState, error) {
	output, err := exec.NewCommand("plumesign", "anisette", "server").CombinedOutput()
	if err != nil {
		log.Err(err).Msg("Error reading anisette server")
		return nil, err
	}
	server, isDefault := parseAnisetteServer(string(output))
	return &AnisetteServerState{Server: server, IsDefault: isDefault}, nil
}

// SetAnisetteServer points the engine at an anisette server. An empty URL
// restores the engine default.
func SetAnisetteServer(server string) error {
	server = strings.TrimSpace(server)
	if server == "" {
		return resetAnisetteServer()
	}
	if err := validateAnisetteServer(server); err != nil {
		return err
	}

	if _, err := exec.NewCommand("plumesign", "anisette", "server", "--set", server).CombinedOutput(); err != nil {
		log.Err(err).Msgf("Error setting anisette server to %s", server)
		return err
	}
	log.Infof("Set anisette server to %s", server)
	return nil
}

func resetAnisetteServer() error {
	if _, err := exec.NewCommand("plumesign", "anisette", "server", "--reset").CombinedOutput(); err != nil {
		log.Err(err).Msg("Error resetting anisette server")
		return err
	}
	log.Infof("Reset anisette server to default")
	return nil
}

// validateAnisetteServer mirrors the engine check, so obviously wrong input is
// rejected before a command runs.
func validateAnisetteServer(server string) error {
	parsed, err := url.Parse(server)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.New("invalid server URL; expected an absolute http:// or https:// URL")
	}
	return nil
}

// parseAnisetteServer extracts the URL from the engine output line
// "[<log prefix>] Anisette server: <URL> (source: ...)". The source suffix
// tells whether the URL is the engine default or a configured one; a default
// server is reported as the empty string, because an unset input means "use
// the default".
func parseAnisetteServer(output string) (string, bool) {
	const marker = "Anisette server:"
	for _, line := range strings.Split(output, "\n") {
		index := strings.Index(line, marker)
		if index < 0 {
			continue
		}
		value := strings.TrimSpace(line[index+len(marker):])
		source := strings.TrimSpace(value)
		isDefault := strings.Contains(source, "(source: default)")
		if cutoff := strings.Index(source, " (source:"); cutoff >= 0 {
			source = source[:cutoff]
		}
		if isDefault {
			return "", true
		}
		if server := strings.TrimSpace(source); server != "" {
			return server, false
		}
	}
	return "", false
}
