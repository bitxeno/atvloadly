package manager

import (
	"errors"
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
