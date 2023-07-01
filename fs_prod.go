//go:build !dev

package main

import (
	"embed"
	"io/fs"
)

//go:embed all:view/dist/*
var view embed.FS

func getViewAssets() fs.FS {
	embed, err := fs.Sub(view, "view/dist")
	if err != nil {
		panic(err)
	}

	return embed
}
