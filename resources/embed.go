package resources

import (
	"embed"
	"io/fs"
	"os"
	"path"

	"github.com/hensi01/play-music/conf"
	"github.com/hensi01/play-music/utils/merge"
)

//go:embed *
var embedFS embed.FS

func FS() fs.FS {
	return merge.FS{
		Base:    embedFS,
		Overlay: os.DirFS(path.Join(conf.Server.DataFolder.String(), "resources")),
	}
}
