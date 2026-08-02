package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var filesystem embed.FS

// BuildAssets returns the built frontend assets (Vite output in dist/).
func BuildAssets() fs.FS {
	dist, _ := fs.Sub(filesystem, "dist")
	return dist
}
