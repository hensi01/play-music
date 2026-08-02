package s3

import (
	"context"
	"strings"

	"github.com/hensi01/play-music/log"
)

// bucketEvents lists the S3 notification events that trigger a library scan.
var bucketEvents = []string{
	"s3:ObjectCreated:*",
	"s3:ObjectRemoved:*",
}

// Start implements storage.Watcher using MinIO's bucket notification listener.
// It only works with MinIO (or other S3 servers that support the
// ListenBucketNotification extension); on plain AWS S3 the watcher simply
// reports no events and the periodic scanner schedule takes over.
func (s *s3Storage) Start(ctx context.Context) (<-chan string, error) {
	ch := make(chan string, 16)
	infoCh := s.client.ListenBucketNotification(ctx, s.bucket, s.prefix, "", bucketEvents)
	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case info, ok := <-infoCh:
				if !ok {
					return
				}
				if info.Err != nil {
					log.Warn(ctx, "S3 watcher: error listening to bucket notifications", "err", info.Err)
					continue
				}
				for _, record := range info.Records {
					key := record.S3.Object.Key
					if s.prefix != "" {
						key = strings.TrimPrefix(key, s.prefix+"/")
					}
					ch <- key
				}
			}
		}
	}()
	return ch, nil
}
