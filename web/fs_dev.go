//go:build dev

package web

import (
	"io/fs"
	"os"
)

func StaticAssets() fs.FS {
	return os.DirFS("static/dist")
}
