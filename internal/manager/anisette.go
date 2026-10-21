package manager

import (
	"github.com/bitxeno/atvloadly/internal/exec"
	"github.com/bitxeno/atvloadly/internal/log"
)

// ResetAnisette deletes the local Anisette provisioning data through the
// engine, so it is generated again on the next login or install. The engine
// owns the location of its data and drops every file that belongs to it; the
// command is local, so it needs no working directory or proxy environment.
func ResetAnisette() error {
	if _, err := exec.NewCommand("plumesign", "anisette", "reset").CombinedOutput(); err != nil {
		log.Err(err).Msg("Error resetting anisette data")
		return err
	}
	log.Infof("Reset anisette data")
	return nil
}
