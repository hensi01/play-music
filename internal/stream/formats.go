package stream

import (
	"sort"
	"strings"
)

// TranscodeFormat is the only container the encoder produces, and
// TranscodeCodec the ffmpeg encoder for it. Both the ffmpeg invocation and the
// stream handler's ?format= validation read them, so adding a second encoder
// target is one edit.
const (
	TranscodeFormat = "mp3"
	TranscodeCodec  = "libmp3lame"
)

type audioFormat struct {
	mime   string
	native bool
}

// formats is the single source of truth for audio containers: which extensions
// are accepted, what content type they carry and whether the browser can play
// them without transcoding. Adding a format here teaches the scanner, the
// upload endpoint and the stream handler at once.
var formats = map[string]audioFormat{
	".mp3":  {"audio/mpeg", true},
	".m4a":  {"audio/mp4", true},
	".aac":  {"audio/aac", true},
	".ogg":  {"audio/ogg", true},
	".opus": {"audio/ogg", true},
	".wav":  {"audio/wav", true},
	".flac": {"audio/flac", true},
	".wma":  {"audio/x-ms-wma", false},
	".aiff": {"audio/aiff", false},
	".aif":  {"audio/aiff", false},
	".wv":   {"audio/wavpack", false},
	".tak":  {"audio/x-tak", false},
	".ape":  {"audio/x-ape", false},
}

func IsAudioExt(ext string) bool {
	_, ok := MIMEForExt(ext)
	return ok
}

func MIMEForExt(ext string) (string, bool) {
	f, ok := formats[strings.ToLower(ext)]
	return f.mime, ok
}

func IsNative(format string) bool {
	return formats["."+strings.ToLower(format)].native
}

// AudioExtensions lists the accepted extensions without the leading dot, in a
// stable order, for clients that need to filter a folder before uploading.
func AudioExtensions() []string {
	exts := make([]string, 0, len(formats))
	for ext := range formats {
		exts = append(exts, strings.TrimPrefix(ext, "."))
	}
	sort.Strings(exts)
	return exts
}
