package stream

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"play-music/internal/config"
)

// Fixed vectors validated against the live Bunny CDN pull zone
// (music.centralcursoss.com.br returned HTTP 200 for these).
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
