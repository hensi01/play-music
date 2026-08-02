package s3

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/minio/minio-go/v7"
)

// s3ReadSeeker implements io.ReadSeeker over an S3 object using ranged GET
// requests. Seeking just closes the current stream and records the new offset;
// the next Read re-opens the object with a Range header, so no data is ever
// downloaded to disk.
type s3ReadSeeker struct {
	mu     sync.Mutex
	client *minio.Client
	bucket string
	key    string
	size   int64
	offset int64
	rc     io.ReadCloser
	closed bool
}

func newS3ReadSeeker(client *minio.Client, bucket, key string, size int64) (*s3ReadSeeker, error) {
	return &s3ReadSeeker{client: client, bucket: bucket, key: key, size: size}, nil
}

func (r *s3ReadSeeker) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return 0, io.ErrClosedPipe
	}
	if r.offset >= r.size {
		return 0, io.EOF
	}
	if r.rc == nil {
		if err := r.open(); err != nil {
			return 0, err
		}
	}
	n, err := r.rc.Read(p)
	r.offset += int64(n)
	if n > 0 && err == io.EOF {
		// A short read with EOF is expected when reaching the end of the object;
		// report the bytes read first.
		err = nil
	}
	if r.offset >= r.size {
		if err == nil {
			err = io.EOF
		}
	}
	return n, err
}

func (r *s3ReadSeeker) Seek(offset int64, whence int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return 0, io.ErrClosedPipe
	}
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = r.offset + offset
	case io.SeekEnd:
		newOffset = r.size + offset
	default:
		return 0, errors.New("s3ReadSeeker: invalid whence")
	}
	if newOffset < 0 {
		return 0, errors.New("s3ReadSeeker: negative position")
	}
	if r.rc != nil {
		_ = r.rc.Close()
		r.rc = nil
	}
	r.offset = newOffset
	return r.offset, nil
}

func (r *s3ReadSeeker) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	if r.rc != nil {
		err := r.rc.Close()
		r.rc = nil
		return err
	}
	return nil
}

// open starts a ranged GET from the current offset. GetObject is lazy: actual
// errors surface on the first Read.
func (r *s3ReadSeeker) open() error {
	opts := minio.GetObjectOptions{}
	// SetRange(start, 0) sends "bytes=N-" which reads from N to the end. For
	// offset 0 we must NOT set a range, otherwise SetRange(0, 0) would send
	// "bytes=0-0" and return only the first byte.
	if r.offset > 0 {
		if err := opts.SetRange(r.offset, 0); err != nil {
			return err
		}
	}
	obj, err := r.client.GetObject(context.Background(), r.bucket, r.key, opts)
	if err != nil {
		return err
	}
	r.rc = obj
	return nil
}
