package stream

import (
	"sort"
	"strings"
	"testing"
)

// AudioExtensions feeds the upload UI, so every entry must be something the
// server actually accepts, written the way the client expects (no leading dot).
func TestAudioExtensionsAreAllAccepted(t *testing.T) {
	exts := AudioExtensions()
	if len(exts) == 0 {
		t.Fatal("no accepted audio extensions")
	}
	if !sort.StringsAreSorted(exts) {
		t.Fatalf("extensions must be sorted for a stable client list, got %v", exts)
	}
	for _, ext := range exts {
		if strings.HasPrefix(ext, ".") {
			t.Errorf("%q must be listed without the leading dot", ext)
		}
		if !IsAudioExt("." + ext) {
			t.Errorf("AudioExtensions lists %q but IsAudioExt rejects it", ext)
		}
	}
}

// These formats decide whether a file is transcoded, which the browser can see:
// an mp3 that got transcoded (or a wma that did not) breaks playback.
func TestNativeFormats(t *testing.T) {
	for _, ext := range []string{".mp3", ".m4a", ".aac", ".ogg", ".opus", ".wav", ".flac"} {
		if !IsNative(strings.TrimPrefix(ext, ".")) {
			t.Errorf("%s should be playable without transcoding", ext)
		}
	}
	for _, ext := range []string{".wma", ".aiff", ".aif", ".wv", ".tak", ".ape"} {
		if IsNative(strings.TrimPrefix(ext, ".")) {
			t.Errorf("%s must be transcoded", ext)
		}
	}
	if IsNative("txt") {
		t.Fatal("unknown format reported as native")
	}
}

func TestMIMEForExtIsCaseInsensitive(t *testing.T) {
	for _, ext := range []string{".mp3", ".MP3", ".Mp3"} {
		if mime, ok := MIMEForExt(ext); !ok || mime != "audio/mpeg" {
			t.Fatalf("MIMEForExt(%q) = %q, %v", ext, mime, ok)
		}
	}
	if _, ok := MIMEForExt(".txt"); ok {
		t.Fatal("non-audio extension accepted")
	}
}
