//go:build embed

package main

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var webFS embed.FS

func getStaticFS() fs.FS {
	sub, _ := fs.Sub(webFS, "dist")
	return sub
}
