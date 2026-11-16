package storage

import (
	"mime"
	"net/url"
	"play-music/internal/config"
	"testing"
	"time"
)

func TestDownloadURLIncludesSignedAttachment(t *testing.T) {
	st, err := New(config.S3Config{Endpoint: "minio.example.com", AccessKey: "user", SecretKey: "secret", Bucket: "music", Region: "us-east-1", Secure: true})
	if err != nil {
		t.Fatal(err)
	}
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": "Canção.mp3"})
	raw, err := st.DownloadURL(t.Context(), "Album/track.mp3", disposition, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("response-content-disposition") != disposition || u.Query().Get("X-Amz-Signature") == "" || u.Query().Get("X-Amz-Expires") != "900" {
		t.Fatalf("attachment not signed correctly: %s", raw)
	}
}
