// Package s3 implements a storage.Storage backend backed by an S3-compatible
// object store (MinIO, AWS S3, etc.). It allows Navidrome to scan and stream
// the music library directly from a bucket.
//
// The library URI uses the "s3" schema, e.g.:
//
//	s3://<bucket>?endpoint=minio.example.com:9000&accessKey=...&secretKey=...&secure=false&prefix=music
//
// Query parameters override the equivalent conf.Server.S3 options.
package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/hensi01/play-music/conf"
	"github.com/hensi01/play-music/core/storage"
	"github.com/hensi01/play-music/core/storage/local"
	"github.com/hensi01/play-music/log"
	"github.com/hensi01/play-music/model/metadata"
)

// Schema is the URL schema registered by this storage backend.
const Schema = "s3"

type s3Storage struct {
	u      url.URL
	client *minio.Client
	bucket string
	prefix string
}

func newS3Storage(u url.URL) storage.Storage {
	q := u.Query()
	endpoint := firstNonEmpty(q.Get("endpoint"), conf.Server.S3.Endpoint)
	accessKey := firstNonEmpty(q.Get("accessKey"), conf.Server.S3.AccessKey)
	secretKey := firstNonEmpty(q.Get("secretKey"), conf.Server.S3.SecretKey)
	bucket := firstNonEmpty(u.Host, conf.Server.S3.Bucket)
	region := firstNonEmpty(q.Get("region"), conf.Server.S3.Region)
	prefix := firstNonEmpty(q.Get("prefix"), conf.Server.S3.Prefix)

	secure := conf.Server.S3.Secure
	if s := q.Get("secure"); s != "" {
		secure = s == "true" || s == "1"
	}

	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		log.Fatal("s3 storage: endpoint, accessKey, secretKey and bucket are required", "uri", u.String())
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
		Region: region,
	})
	if err != nil {
		log.Fatal("s3 storage: error creating client", "endpoint", endpoint, "err", err)
	}
	if conf.Server.S3.Debug {
		client.TraceOn(traceWriter{})
	}

	return &s3Storage{
		u:      u,
		client: client,
		bucket: bucket,
		prefix: strings.Trim(prefix, "/"),
	}
}

// traceWriter routes minio-go HTTP tracing to the Navidrome debug log. Enable
// with ND_S3_DEBUG=true to diagnose connectivity/auth issues with the origin.
type traceWriter struct{}

func (traceWriter) Write(p []byte) (int, error) {
	log.Debug("s3 trace", "request", string(p))
	return len(p), nil
}

// FS verifies the bucket is reachable and returns a MusicFS backed by S3.
func (s *s3Storage) FS() (storage.MusicFS, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return nil, fmt.Errorf("s3 storage: error checking bucket %q: %w", s.bucket, err)
	}
	if !exists {
		return nil, fmt.Errorf("s3 storage: bucket %q does not exist", s.bucket)
	}
	fsys := &s3FS{storage: s}
	fsys.extractor = local.NewExtractor(fsys, s.prefix)
	return fsys, nil
}

func init() {
	storage.Register(Schema, newS3Storage)
}

// key returns the object key for a library-relative path. The FS paths use
// forward slashes (fs.ValidPath semantics); the root is "." which maps to the
// configured prefix (or the bucket root when the prefix is empty).
func (s *s3Storage) key(name string) string {
	name = strings.Trim(name, "/")
	if name == "." || name == "" {
		return s.prefix
	}
	if s.prefix != "" {
		return s.prefix + "/" + name
	}
	return name
}

// pathExists reports whether any object exists under the given key/prefix.
func (s *s3Storage) pathExists(prefix string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
		MaxKeys:   1,
	}
	for obj := range s.client.ListObjects(ctx, s.bucket, opts) {
		if obj.Err != nil {
			return false
		}
		return true
	}
	return false
}

// s3FS implements storage.MusicFS backed by an S3-compatible object store.
type s3FS struct {
	storage   *s3Storage
	extractor local.Extractor
}

func (s *s3FS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	// The root directory always exists: it maps to the configured prefix (or
	// the bucket root), and the bucket reachability was already verified when
	// the FS was created. An empty bucket is a valid (empty) library.
	if name == "." {
		return s.openDir(name)
	}
	key := s.storage.key(name)

	// Try to open as a file first.
	info, err := s.statObject(key)
	if err == nil {
		if info.Size == 0 && strings.HasSuffix(key, "/") {
			// directory placeholder object; treat as a directory
			return s.openDir(name)
		}
		return s.openFile(name, info)
	}
	if !isNoSuchKey(err) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}

	// Not a file. Check if it is a directory prefix.
	dirPrefix := s.dirPrefix(name, key)
	if s.storage.pathExists(dirPrefix) {
		return s.openDir(name)
	}

	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

// dirPrefix returns the object prefix used to test/read a directory. The root
// maps to the configured prefix itself (possibly empty for the bucket root).
func (s *s3FS) dirPrefix(name, key string) string {
	if name == "." {
		return s.storage.prefix
	}
	if key == "" {
		return ""
	}
	return key + "/"
}

func (s *s3FS) openFile(name string, info minio.ObjectInfo) (fs.File, error) {
	r, err := newS3ReadSeeker(s.storage.client, s.storage.bucket, info.Key, info.Size)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	return &s3File{reader: r, info: objectFileInfo{name: path.Base(name), info: info}}, nil
}

func (s *s3FS) openDir(name string) (fs.File, error) {
	entries, err := s.ReadDir(name)
	if err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: err}
	}
	return &s3Dir{name: name, entries: entries}, nil
}

// ReadDir lists the immediate children (files and subdirectories) of name.
func (s *s3FS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	key := s.storage.key(name)
	prefix := s.dirPrefix(name, key)

	entries := map[string]fs.DirEntry{}
	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: false,
	}
	ctx := context.Background()
	for obj := range s.storage.client.ListObjects(ctx, s.storage.bucket, opts) {
		if obj.Err != nil {
			return nil, &fs.PathError{Op: "readdir", Path: name, Err: obj.Err}
		}
		rel := strings.TrimPrefix(obj.Key, prefix)
		rel = strings.TrimSuffix(rel, "/")
		if rel == "" {
			continue
		}
		// Common prefixes (directories) are returned by minio-go with a
		// trailing "/".
		isDir := strings.HasSuffix(obj.Key, "/")
		if isDir {
			dirName := strings.SplitN(rel, "/", 2)[0]
			entries[dirName] = dirEntry{name: dirName, isDir: true}
		} else {
			entries[rel] = dirEntry{name: rel, size: obj.Size, modTime: obj.LastModified}
		}
	}

	if len(entries) == 0 && name != "." {
		// Distinguish an empty directory from a non-existent one.
		if !s.storage.pathExists(prefix) {
			return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
		}
	}
	res := make([]fs.DirEntry, 0, len(entries))
	for _, e := range entries {
		res = append(res, e)
	}
	return res, nil
}

func (s *s3FS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	// The root directory always exists: it maps to the configured prefix (or
	// the bucket root), whose reachability was verified when the FS was created.
	if name == "." {
		return dirInfo{name: "."}, nil
	}
	key := s.storage.key(name)
	info, err := s.statObject(key)
	if err == nil {
		return objectFileInfo{name: path.Base(name), info: info}, nil
	}
	if !isNoSuchKey(err) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}
	if s.storage.pathExists(s.dirPrefix(name, key)) {
		return dirInfo{name: path.Base(name)}, nil
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

func (s *s3FS) ReadFile(name string) ([]byte, error) {
	f, err := s.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func (s *s3FS) statObject(key string) (minio.ObjectInfo, error) {
	if key == "" {
		return minio.ObjectInfo{}, minioError{msg: "no such key"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.storage.client.StatObject(ctx, s.storage.bucket, key, minio.StatObjectOptions{})
}

// ReadTags extracts metadata from the given files using the registered
// extractor, filling in FileInfo from the object store when needed.
func (s *s3FS) ReadTags(paths ...string) (map[string]metadata.Info, error) {
	res, err := s.extractor.Parse(paths...)
	if err != nil {
		return nil, err
	}
	for p, v := range res {
		if v.FileInfo == nil {
			info, err := s.Stat(p)
			if err != nil {
				return nil, err
			}
			// The concrete types returned by Stat implement metadata.FileInfo
			// (they provide BirthTime), so they can be used directly by the
			// tag extraction pipeline.
			v.FileInfo = info.(metadata.FileInfo)
			res[p] = v
		}
	}
	return res, nil
}

// s3File is an fs.File backed by a seekable S3 object reader. It implements
// io.ReadSeeker so tag parsers (e.g. go-taglib) can work directly on the
// object stream without downloading it to disk.
type s3File struct {
	reader *s3ReadSeeker
	info   fs.FileInfo
}

func (f *s3File) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *s3File) Read(p []byte) (int, error) { return f.reader.Read(p) }
func (f *s3File) Seek(offset int64, whence int) (int64, error) {
	return f.reader.Seek(offset, whence)
}
func (f *s3File) Close() error { return f.reader.Close() }

// s3Dir is an fs.File representing a directory listing from S3.
type s3Dir struct {
	name    string
	pos     int
	entries []fs.DirEntry
}

func (d *s3Dir) Stat() (fs.FileInfo, error) { return dirInfo{name: path.Base(d.name)}, nil }
func (d *s3Dir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: errors.New("is a directory")}
}
func (d *s3Dir) Close() error { return nil }
func (d *s3Dir) ReadDir(n int) ([]fs.DirEntry, error) {
	if n <= 0 {
		res := d.entries[d.pos:]
		d.pos = len(d.entries)
		return res, nil
	}
	if d.pos >= len(d.entries) {
		return nil, io.EOF
	}
	end := min(d.pos+n, len(d.entries))
	res := d.entries[d.pos:end]
	d.pos = end
	return res, nil
}

// firstNonEmpty returns the first non-empty string in the list.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// isNoSuchKey reports whether the error means the object does not exist.
func isNoSuchKey(err error) bool {
	if err == nil {
		return false
	}
	if err == fs.ErrNotExist {
		return true
	}
	var er minio.ErrorResponse
	if errors.As(err, &er) {
		return er.Code == "NoSuchKey" || er.Code == "NotFound" || er.StatusCode == 404
	}
	return strings.Contains(strings.ToLower(err.Error()), "no such key")
}

// minioError is a minimal error type used for the empty-key case.
type minioError struct{ msg string }

func (e minioError) Error() string { return e.msg }
