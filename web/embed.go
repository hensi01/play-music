package web

import (
	"embed"
	"io/fs"
)

//go:embed all:assets
var filesystem embed.FS

// BuildAssets returns the web UI assets (HTML/CSS/JS, embedded in the binary).
func BuildAssets() fs.FS {
	dist, _ := fs.Sub(filesystem, "assets")
	return dist
}
