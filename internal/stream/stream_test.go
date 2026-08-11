package stream

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"play-music/internal/config"
	"play-music/internal/model"
)

// Signing vectors: the HS256/MD5 math is validated offline against fixed
// vectors. NOTE: these vectors use a LEGACY token key — the live pull zone
// now uses the key from .env (ND_CDN_TOKENAUTHKEY), so a live request with
// these vectors returns 403. The signing algorithm itself is unchanged
// (validated live on 2026-08-11 with the production key).
const (
	testKey      = "50df94e8-7424-4a4f-99e9-1c136831052d"
	testPath     = "/Test Artist/Test Album/test.mp3"
	testExpires  = "1786072596"
	expectedHS25 = "HS256-ZQDIooiFFM7aYHPmKb6KYR6jtbTP6cQTZJEOjP1TqbA"
	expectedMD5  = "WOgdbUhlRrRq_pBXwjZuSQ"
)

func testService(advanced bool) *Service {
	return &Service{
		cfg: &config.Config{
			CDNTokenKey:     testKey,
			CDNAdvancedAuth: advanced,
		},
		log: slog.Default(),
	}
}

func TestSignAdvanced(t *testing.T) {
	got := testService(true).sign(testPath, testExpires)
	if got != expectedHS25 {
		t.Fatalf("HS256 token mismatch: got %q want %q", got, expectedHS25)
	}
}

func TestSignBasic(t *testing.T) {
	got := testService(false).sign(testPath, testExpires)
	if got != expectedMD5 {
		t.Fatalf("MD5 token mismatch: got %q want %q", got, expectedMD5)
	}
}

func TestCDNURLEncodesPath(t *testing.T) {
	s := testService(true)
	s.cfg.CDNBaseURL = "https://music.example.com"
	s.cfg.CDNTokenTTL = 24 * time.Hour

	u := s.CDNURL("Test Artist/Test Album/test.mp3")
	if !strings.Contains(u, "/Test%20Artist/Test%20Album/test.mp3?") {
		t.Fatalf("path not percent-encoded: %q", u)
	}
	if !strings.Contains(u, "&expires=") || !strings.Contains(u, "token=HS256-") {
		t.Fatalf("unexpected URL: %q", u)
	}
}

func TestCDNURLPathPrefix(t *testing.T) {
	s := testService(false)
	s.cfg.CDNBaseURL = "https://music.example.com/"
	s.cfg.CDNPathPrefix = "library/"
	s.cfg.CDNTokenTTL = 24 * time.Hour

	u := s.CDNURL("album/song.mp3")
	if !strings.Contains(u, "https://music.example.com/library/album/song.mp3?") {
		t.Fatalf("prefix not applied: %q", u)
	}
}

// CDN-only policy: with the CDN disabled or misconfigured, StreamURL and
// SignedURL must error — there is no presigned/local fallback anymore.
func TestStreamURLErrorsWithoutCDN(t *testing.T) {
	cfg := &config.Config{}
	s := &Service{cfg: cfg, log: slog.Default()}
	song := &model.Song{Path: "uploads/a.mp3"}

	if _, err := s.StreamURL(t.Context(), song); err == nil {
		t.Fatal("StreamURL must error when CDN is disabled")
	}
	if _, err := s.SignedURL("uploads/a.mp3"); err == nil {
		t.Fatal("SignedURL must error when CDN is disabled")
	}
}

// With the CDN fully configured, StreamURL/SignedURL return a signed URL that
// starts with the base URL (no fallback to presigned MinIO URLs).
func TestStreamURLUsesCDNWhenConfigured(t *testing.T) {
	s := testService(true)
	s.cfg.CDNEnabled = true
	s.cfg.CDNBaseURL = "https://music.example.com"
	s.cfg.CDNTokenTTL = 24 * time.Hour
	song := &model.Song{Path: "uploads/a.mp3"}

	u, err := s.StreamURL(t.Context(), song)
	if err != nil {
		t.Fatalf("StreamURL: %v", err)
	}
	if !strings.HasPrefix(u, "https://music.example.com/uploads/a.mp3?") {
		t.Fatalf("StreamURL not a CDN URL: %q", u)
	}

	ku, err := s.SignedURL("uploads/k.mp4")
	if err != nil {
		t.Fatalf("SignedURL: %v", err)
	}
	if !strings.HasPrefix(ku, "https://music.example.com/uploads/k.mp4?") {
		t.Fatalf("SignedURL not a CDN URL: %q", ku)
	}
}
