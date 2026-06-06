//go:build !embed

package main

import (
	"io/fs"
	"os"
)

func getStaticFS() fs.FS {
	return os.DirFS("web/dist")
}
