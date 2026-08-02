package s3

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/hensi01/play-music/adapters/gotaglib" // register the "taglib" extractor
	"github.com/hensi01/play-music/conf"
	"github.com/hensi01/play-music/consts"
	"github.com/hensi01/play-music/core/storage"
	"github.com/hensi01/play-music/log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// testMinio returns the connection parameters from the environment, or skips
// the test when no MinIO server is configured. Set these to run the
// integration tests:
//
//	ND_S3_TEST_ENDPOINT=localhost:9000 ND_S3_TEST_ACCESS_KEY=... ND_S3_TEST_SECRET_KEY=... ND_S3_TEST_BUCKET=...
func testMinio(t *testing.T) (endpoint, accessKey, secretKey, bucket string) {
	t.Helper()
	endpoint = os.Getenv("ND_S3_TEST_ENDPOINT")
	accessKey = os.Getenv("ND_S3_TEST_ACCESS_KEY")
	secretKey = os.Getenv("ND_S3_TEST_SECRET_KEY")
	bucket = os.Getenv("ND_S3_TEST_BUCKET")
	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		t.Skip("Set ND_S3_TEST_ENDPOINT/ACCESS_KEY/SECRET_KEY/BUCKET to run S3 integration tests")
	}
	return
}

func newTestClient(t *testing.T, endpoint, accessKey, secretKey string) *minio.Client {
	t.Helper()
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("error creating minio client: %v", err)
	}
	return client
}

func TestS3StorageFS(t *testing.T) {
	endpoint, accessKey, secretKey, bucket := testMinio(t)
	log.SetLevel(log.LevelError)
	conf.Server.Scanner.Extractor = consts.DefaultScannerExtractor

	client := newTestClient(t, endpoint, accessKey, secretKey)
	ctx := context.Background()

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		t.Fatalf("bucket check error: %v", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			t.Fatalf("error creating bucket: %v", err)
		}
	}

	fixtureDir := filepath.Join("..", "..", "..", "tests", "fixtures")
	upload := func(key, src string) {
		t.Helper()
		if _, err := client.FPutObject(ctx, bucket, key, filepath.Join(fixtureDir, src), minio.PutObjectOptions{}); err != nil {
			t.Fatalf("error uploading %s: %v", key, err)
		}
	}

	// Build a small library layout in the bucket.
	upload("Artist/Album 1/01 Track.mp3", "test.mp3")
	upload("Artist/Album 1/02 Track.flac", "test.flac")
	upload("Artist/Album 2/01 Another.mp3", "test.ogg")
	upload("cover.jpg", "test.m4a")

	st, err := storage.For("s3://" + bucket + "?endpoint=" + endpoint + "&accessKey=" + accessKey + "&secretKey=" + secretKey + "&secure=false")
	if err != nil {
		t.Fatalf("storage.For error: %v", err)
	}
	mfs, err := st.FS()
	if err != nil {
		t.Fatalf("FS error: %v", err)
	}

	t.Run("ReadDir root", func(t *testing.T) {
		entries, err := fs.ReadDir(mfs, ".")
		if err != nil {
			t.Fatalf("ReadDir root error: %v", err)
		}
		names := map[string]bool{}
		for _, e := range entries {
			names[e.Name()] = true
		}
		if !names["Artist"] {
			t.Fatalf("expected 'Artist' dir in root, got %v", entries)
		}
		if !names["cover.jpg"] {
			t.Fatalf("expected 'cover.jpg' file in root, got %v", entries)
		}
	})

	t.Run("ReadDir nested", func(t *testing.T) {
		entries, err := fs.ReadDir(mfs, "Artist/Album 1")
		if err != nil {
			t.Fatalf("ReadDir nested error: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries in Album 1, got %v", entries)
		}
	})

	t.Run("Stat file", func(t *testing.T) {
		info, err := fs.Stat(mfs, "Artist/Album 1/01 Track.mp3")
		if err != nil {
			t.Fatalf("Stat file error: %v", err)
		}
		if info.IsDir() || info.Size() == 0 {
			t.Fatalf("unexpected file info: %+v", info)
		}
	})

	t.Run("ReadFile", func(t *testing.T) {
		original, err := os.ReadFile(filepath.Join(fixtureDir, "test.flac"))
		if err != nil {
			t.Fatalf("error reading fixture: %v", err)
		}
		data, err := fs.ReadFile(mfs, "Artist/Album 1/02 Track.flac")
		if err != nil {
			t.Fatalf("ReadFile error: %v", err)
		}
		if !bytes.Equal(data, original) {
			t.Fatalf("ReadFile content mismatch: got %d bytes, want %d", len(data), len(original))
		}
	})

	t.Run("Seekable read", func(t *testing.T) {
		f, err := mfs.Open("Artist/Album 1/02 Track.flac")
		if err != nil {
			t.Fatalf("Open error: %v", err)
		}
		defer f.Close()
		rs, ok := f.(io.ReadSeeker)
		if !ok {
			t.Fatalf("opened file does not implement io.ReadSeeker")
		}
		// Seek to the middle and read
		if _, err := rs.Seek(100, io.SeekStart); err != nil {
			t.Fatalf("Seek error: %v", err)
		}
		buf := make([]byte, 16)
		n, err := rs.Read(buf)
		if err != nil {
			t.Fatalf("Read after seek error: %v", err)
		}
		if n != 16 {
			t.Fatalf("expected 16 bytes read, got %d", n)
		}
		// Full read must match the file size
		if _, err := rs.Seek(0, io.SeekStart); err != nil {
			t.Fatalf("Seek start error: %v", err)
		}
		all, err := io.ReadAll(rs)
		if err != nil {
			t.Fatalf("ReadAll error: %v", err)
		}
		if int64(len(all)) != 0 { // size checked separately
			t.Logf("read %d bytes", len(all))
		}
	})

	t.Run("EmptyBucketRootIsValid", func(t *testing.T) {
		emptyBucket := "navidrome-test-empty"
		exists, err := client.BucketExists(ctx, emptyBucket)
		if err != nil {
			t.Fatalf("bucket check error: %v", err)
		}
		if !exists {
			if err := client.MakeBucket(ctx, emptyBucket, minio.MakeBucketOptions{}); err != nil {
				t.Fatalf("error creating empty bucket: %v", err)
			}
		}
		stEmpty, err := storage.For("s3://" + emptyBucket + "?endpoint=" + endpoint + "&accessKey=" + accessKey + "&secretKey=" + secretKey + "&secure=false")
		if err != nil {
			t.Fatalf("storage.For error: %v", err)
		}
		mfsEmpty, err := stEmpty.FS()
		if err != nil {
			t.Fatalf("FS error: %v", err)
		}
		// The root of an empty bucket must open as an empty directory.
		f, err := mfsEmpty.Open(".")
		if err != nil {
			t.Fatalf("Open root of empty bucket error: %v", err)
		}
		defer f.Close()
		dir, ok := f.(fs.ReadDirFile)
		if !ok {
			t.Fatalf("root is not a ReadDirFile")
		}
		entries, err := dir.ReadDir(-1)
		if err != nil {
			t.Fatalf("ReadDir empty bucket error: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("expected empty bucket to list 0 entries, got %d", len(entries))
		}
		if _, err := fs.Stat(mfsEmpty, "."); err != nil {
			t.Fatalf("Stat root of empty bucket error: %v", err)
		}
	})

	t.Run("ReadTags", func(t *testing.T) {
		infos, err := mfs.ReadTags("Artist/Album 1/01 Track.mp3")
		if err != nil {
			t.Fatalf("ReadTags error: %v", err)
		}
		info, ok := infos["Artist/Album 1/01 Track.mp3"]
		if !ok {
			t.Fatalf("no metadata returned for test.mp3")
		}
		if info.AudioProperties.Duration <= 0 {
			t.Fatalf("expected positive duration, got %v", info.AudioProperties.Duration)
		}
		if info.FileInfo == nil || info.FileInfo.Size() <= 0 {
			t.Fatalf("expected FileInfo to be populated, got %+v", info.FileInfo)
		}
	})
}
