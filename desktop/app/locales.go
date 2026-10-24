package main

import (
	"embed"

	"github.com/bitxeno/atvloadly/internal/i18n"
)

// The server's message catalogs, copied from the repo root locales/ by the
// sync-locales build step (mygo.config.ts buildCommand). The root main's
// embed is not reachable from this module, so the desktop app carries its
// own copy, like the built web frontend does.
//
//go:embed locales
var localesFs embed.FS

func initI18n() {
	i18n.Init(localesFs)
}
