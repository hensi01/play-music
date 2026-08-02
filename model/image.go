package model

import (
	"path/filepath"

	"github.com/hensi01/play-music/conf"
	"github.com/hensi01/play-music/consts"
)

// UploadedImagePath returns the absolute filesystem path for a manually uploaded
// entity cover image. Returns empty string if filename is empty.
func UploadedImagePath(entityType, filename string) string {
	if filename == "" {
		return ""
	}
	return filepath.Join(conf.Server.DataFolder.String(), consts.ArtworkFolder, entityType, filename)
}
