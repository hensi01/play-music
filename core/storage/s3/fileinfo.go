package s3

import (
	"io/fs"
	"path"
	"time"

	"github.com/minio/minio-go/v7"
)

// objectFileInfo adapts minio.ObjectInfo to fs.FileInfo. It also implements
// BirthTime so it satisfies metadata.FileInfo, which the tag extraction
// pipeline expects.
type objectFileInfo struct {
	name string
	info minio.ObjectInfo
}

func (o objectFileInfo) Name() string {
	if o.name != "" {
		return o.name
	}
	return path.Base(o.info.Key)
}

func (o objectFileInfo) Size() int64        { return o.info.Size }
func (o objectFileInfo) Mode() fs.FileMode  { return 0o644 }
func (o objectFileInfo) ModTime() time.Time { return o.info.LastModified }
func (o objectFileInfo) IsDir() bool        { return false }
func (o objectFileInfo) Sys() any           { return &o.info }

func (o objectFileInfo) BirthTime() time.Time {
	return o.info.LastModified
}

// dirInfo is a synthetic fs.FileInfo for directories that have no physical
// object of their own (S3 has no real directories).
type dirInfo struct {
	name string
}

func (d dirInfo) Name() string       { return d.name }
func (d dirInfo) Size() int64        { return 0 }
func (d dirInfo) Mode() fs.FileMode  { return fs.ModeDir | 0o755 }
func (d dirInfo) ModTime() time.Time { return time.Time{} }
func (d dirInfo) IsDir() bool        { return true }
func (d dirInfo) Sys() any           { return nil }

func (d dirInfo) BirthTime() time.Time { return time.Time{} }

// dirEntry adapts an fs.FileInfo to fs.DirEntry.
type dirEntry struct {
	name    string
	isDir   bool
	size    int64
	modTime time.Time
}

func (d dirEntry) Name() string { return d.name }
func (d dirEntry) IsDir() bool  { return d.isDir }
func (d dirEntry) Type() fs.FileMode {
	if d.isDir {
		return fs.ModeDir
	}
	return 0
}
func (d dirEntry) Info() (fs.FileInfo, error) {
	if d.isDir {
		return dirInfo{name: d.name}, nil
	}
	return objectFileInfo{
		name: d.name,
		info: minio.ObjectInfo{Key: d.name, Size: d.size, LastModified: d.modTime},
	}, nil
}
