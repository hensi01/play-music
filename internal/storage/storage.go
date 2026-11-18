package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
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

func (s *Storage) List(ctx context.Context) <-chan minio.ObjectInfo {
	return s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    "",
		Recursive: true,
	})
}

// Stat returns the object metadata. The caching proxy in front of MinIO
// occasionally answers Access Denied for a few seconds after idle time, so
// transient errors are retried with a short backoff.
func (s *Storage) Stat(ctx context.Context, key string) (minio.ObjectInfo, error) {
	var last error
	for i := 0; i < 4; i++ {
		info, err := s.statOnce(ctx, key)
		if err == nil {
			return info, nil
		}
		last = err
		if i < 3 && isTransientStat(err) {
			select {
			case <-time.After(time.Duration(i+1) * time.Second):
			case <-ctx.Done():
				return minio.ObjectInfo{}, ctx.Err()
			}
			continue
		}
		break
	}
	return minio.ObjectInfo{}, last
}

func (s *Storage) statOnce(ctx context.Context, key string) (minio.ObjectInfo, error) {
	return s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
}

// WaitForObject polls until a freshly written object is readable. The caching
// proxy can take seconds to expose it (observed ~6s), and a not-found answer is
// the expected state in that window, so the wait owns the polling policy and
// uses a single attempt per poll instead of nesting Stat's transient retry.
func (s *Storage) WaitForObject(ctx context.Context, key string) {
	deadline := time.Now().Add(objectVisibilityTimeout)
	for {
		if _, err := s.statOnce(ctx, key); err == nil {
			return
		}
		if ctx.Err() != nil || time.Now().After(deadline) {
			return
		}
		select {
		case <-time.After(objectVisibilityPoll):
		case <-ctx.Done():
			return
		}
	}
}

const (
	objectVisibilityTimeout = 60 * time.Second
	objectVisibilityPoll    = 2 * time.Second
)

func isTransientStat(err error) bool {
	var resp minio.ErrorResponse
	if !errors.As(err, &resp) {
		return false
	}
	switch resp.Code {
	case "AccessDenied", "RequestTimeout", "SlowDown", "InternalError":
		return true
	}
	return false
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

func (s *Storage) PresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiry, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *Storage) DownloadURL(ctx context.Context, key, disposition string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiry, url.Values{"response-content-disposition": {disposition}})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *Storage) Put(ctx context.Context, key string, size int64, contentType string, reader io.Reader) error {
	if size < 0 {
		size = 0
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, size,
		minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *Storage) Remove(ctx context.Context, key string) error {
	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err == nil {
		return nil
	}
	var resp minio.ErrorResponse
	if errors.As(err, &resp) && (resp.Code == "NoSuchKey" || resp.Code == "NotFound") {
		return nil
	}
	return err
}
