package config

import (
	"testing"
	"time"
)

func TestStreamURLTTL(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  time.Duration
	}{{"", time.Hour}, {"1h", time.Hour}, {"168h", 7 * 24 * time.Hour}, {"169h", time.Hour}, {"0", time.Hour}, {"invalid", time.Hour}} {
		t.Run(tc.input, func(t *testing.T) {
			t.Setenv("ND_STREAM_URL_TTL", tc.input)
			if got := Load().StreamURLTTL; got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestScannerPruningRequiresExplicitOptIn(t *testing.T) {
	t.Setenv("ND_SCANNER_DELETE_MISSING", "")
	if Load().ScannerDeleteMissing {
		t.Fatal("missing objects must not be deleted by default")
	}
	t.Setenv("ND_SCANNER_DELETE_MISSING", "true")
	if !Load().ScannerDeleteMissing {
		t.Fatal("explicit opt-in ignored")
	}
}
