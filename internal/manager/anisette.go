package manager

import (
	"errors"
	"net/url"
	"regexp"
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

// RemoveAnisette deletes an account's Anisette data directory through the
// engine. It is used for data left behind by an account that no longer exists
// in the account list: there is no account to re-login for, so the leftover
// data is deleted outright instead of reset.
func RemoveAnisette(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return errors.New("account email is required")
	}

	if _, err := exec.NewCommand("plumesign", "anisette", "remove", "-u", email).CombinedOutput(); err != nil {
		log.Err(err).Msgf("Error removing anisette data for %s", email)
		return err
	}
	log.Infof("Removed anisette data for %s", email)
	return nil
}

// AnisetteAccount is one account that has Anisette data stored on disk. The
// engine provisions this data per account, and a failed login can leave data
// behind for an account that is no longer in the account list, so the engine's
// anisette list — not the account list — decides which accounts can be reset.
type AnisetteAccount struct {
	Email  string `json:"email"`
	Name   string `json:"name"`
	TeamID string `json:"team_id"`
	// Orphan is true when the account is not in the account list (accounts.json).
	// Its data is leftover from a failed or removed login, so it can only be
	// deleted, not reset.
	Orphan bool `json:"orphan"`
}

// AnisetteAccounts lists the accounts that have Anisette data stored, by asking
// the engine. Unlike the account list, this includes accounts whose login
// failed but whose data directory still exists; those are marked orphaned so
// the caller knows they can only be deleted.
func AnisetteAccounts() ([]AnisetteAccount, error) {
	output, err := exec.NewCommand("plumesign", "anisette", "list").CombinedOutput()
	if err != nil {
		log.Err(err).Msg("Error listing anisette accounts")
		return nil, err
	}
	accounts := parseAnisetteAccounts(string(output))

	known := map[string]bool{}
	if stored, err := accountManager.GetAccounts(); err == nil {
		for email := range stored.Accounts {
			known[email] = true
		}
	}
	for i := range accounts {
		accounts[i].Orphan = !known[accounts[i].Email]
	}
	return accounts, nil
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

// anisetteAccountRow splits one row of `plumesign anisette list` on runs of two
// or more spaces. The table aligns its columns with spaces, but a value such as
// the NAME field can itself contain a single space, so only a run of spaces is
// treated as the column separator. This keeps the email intact even when a name
// contains spaces.
var anisetteAccountRow = regexp.MustCompile(`\s{2,}`)

// parseAnisetteAccounts reads the fixed-width table printed by
// `plumesign anisette list`:
//
//	Accounts with Anisette data:
//	ACCOUNT                           NAME                  TEAM ID
//	user@example.com                  wu                    8245MZ34WA
//	other@example.com                 -                     -
//	8478ef9a3673f48ff6840ac7c86c519a  -                     -
//
// The first line is a heading and the second is the column header; the rest are
// rows. A blank NAME or TEAM ID is printed as "-", which is normalized to empty.
// The ACCOUNT column is the email when the engine can map the data back to an
// account, or the anisette directory hash when it cannot (data left behind by a
// failed login); every data row is kept.
func parseAnisetteAccounts(output string) []AnisetteAccount {
	accounts := []AnisetteAccount{}
	headerSeen := false
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := anisetteAccountRow.Split(strings.TrimSpace(line), -1)
		if len(fields) < 1 {
			continue
		}
		// Skip everything up to and including the column header.
		if !headerSeen {
			if fields[0] == "ACCOUNT" {
				headerSeen = true
			}
			continue
		}

		account := AnisetteAccount{Email: strings.TrimSpace(fields[0])}
		if len(fields) > 1 {
			account.Name = normalizeAnisetteCell(fields[1])
		}
		if len(fields) > 2 {
			account.TeamID = normalizeAnisetteCell(fields[2])
		}
		accounts = append(accounts, account)
	}
	return accounts
}

// normalizeAnisetteCell trims a cell and maps the engine's "-" placeholder for
// an empty value to an empty string.
func normalizeAnisetteCell(value string) string {
	value = strings.TrimSpace(value)
	if value == "-" {
		return ""
	}
	return value
}
