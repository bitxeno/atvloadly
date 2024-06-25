//go:build !dev

package web

import (
	"embed"
	"io/fs"
)

//go:embed all:static/dist/*
var static embed.FS

func StaticAssets() fs.FS {
	embed, err := fs.Sub(static, "static/dist")
	if err != nil {
		panic(err)
	}

	return embed
}
