//go:build dev

package main

import (
	"io/fs"
	"os"
)

func getViewAssets() fs.FS {
	return os.DirFS("view/dist")
}
