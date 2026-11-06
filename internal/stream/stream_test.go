package stream

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"play-music/internal/config"
	"play-music/internal/model"
	"play-music/internal/storage"
)

func TestStreamURLSignsConfiguredStorageObject(t *testing.T) {
	st, err := storage.New(config.S3Config{Endpoint: "minio.example.com", AccessKey: "user", SecretKey: "secret", Bucket: "music", Region: "us-east-1", Secure: true})
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{cfg: &config.Config{StreamURLTTL: 24 * time.Hour}, st: st}
	raw, err := s.StreamURL(t.Context(), &model.Song{Path: "Artist Name/Album/test.mp3"})
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "https" || u.Host != "minio.example.com" || u.EscapedPath() != "/music/Artist%20Name/Album/test.mp3" {
		t.Fatalf("unexpected destination: %s %s %s", u.Scheme, u.Host, u.EscapedPath())
	}
	if u.Query().Get("X-Amz-Expires") != "86400" || u.Query().Get("X-Amz-Signature") == "" {
		t.Fatal("missing signature or wrong expiration")
	}
	if strings.Contains(raw, "secret") {
		t.Fatal("storage secret exposed in URL")
	}
}

func TestStreamURLErrorForInvalidTTL(t *testing.T) {
	st, err := storage.New(config.S3Config{Endpoint: "minio.example.com", AccessKey: "user", SecretKey: "secret", Bucket: "music", Region: "us-east-1", Secure: true})
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{cfg: &config.Config{StreamURLTTL: 8 * 24 * time.Hour}, st: st}
	if _, err := s.StreamURL(t.Context(), &model.Song{Path: "a.mp3"}); err == nil {
		t.Fatal("invalid URL lifetime was accepted")
	}
}
