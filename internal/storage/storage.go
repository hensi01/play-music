package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"play-music/internal/config"
)

type Storage struct {
	client *minio.Client
	bucket string
}

func New(cfg config.S3Config) (*Storage, error) {
	if cfg.Endpoint == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, errors.New("configuração S3 incompleta (endpoint/accessKey/secretKey)")
	}
	if cfg.Bucket == "" {
		cfg.Bucket = "play-music"
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.Secure,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("cliente S3: %w", err)
	}
	return &Storage{client: client, bucket: cfg.Bucket}, nil
}

func (s *Storage) Bucket() string { return s.bucket }

// List returns a channel of objects under the bucket root. Per-item errors
// surface through ObjectInfo.Err.
func (s *Storage) List(ctx context.Context) <-chan minio.ObjectInfo {
	return s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    "",
		Recursive: true,
	})
}

func (s *Storage) Stat(ctx context.Context, key string) (minio.ObjectInfo, error) {
	return s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
}

func (s *Storage) ObjectExists(ctx context.Context, key string) (bool, error) {
	_, err := s.Stat(ctx, key)
	if err == nil {
		return true, nil
	}
	var resp minio.ErrorResponse
	if errors.As(err, &resp) && (resp.Code == "NoSuchKey" || resp.Code == "NotFound") {
		return false, nil
	}
	return false, err
}

// Open returns a reader for the object (supports range via options).
func (s *Storage) Open(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error) {
	opts := minio.GetObjectOptions{}
	if offset > 0 || length > 0 {
		if err := opts.SetRange(offset, length); err != nil {
			return nil, err
		}
	}
	obj, err := s.client.GetObject(ctx, s.bucket, key, opts)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (s *Storage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.Open(ctx, key, 0, -1)
}

func (s *Storage) PresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiry, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// Put uploads an object to the bucket (used by the admin song upload).
func (s *Storage) Put(ctx context.Context, key string, size int64, contentType string, reader io.Reader) error {
	if size < 0 {
		size = 0
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, size,
		minio.PutObjectOptions{ContentType: contentType})
	return err
}

// keyFromURL extracts the object key from a URL path (e.g. "/folder/file.mp3").
func keyFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return strings.TrimLeft(raw, "/")
	}
	return strings.TrimLeft(u.Path, "/")
}
