package lyrics

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"play-music/internal/model"
	"play-music/internal/storage"
	"play-music/internal/store"
)

type Service struct {
	store *store.Store
	st    *storage.Storage
	log   *slog.Logger
}

func New(st *store.Store, str *storage.Storage, log *slog.Logger) *Service {
	return &Service{store: st, st: str, log: log}
}

var lrcTimeRe = regexp.MustCompile(`\[(\d{1,3}):(\d{1,2})(?:[.:](\d{1,3}))?\]`)
var lrcOffsetRe = regexp.MustCompile(`\[offset:\s*([+-]?\d+)\s*\]`)

// Lookup returns the lyrics for a song: embedded tags first (unsynced), then a
// sibling .lrc file in the same S3 folder (synced).
func (s *Service) Lookup(ctx context.Context, songID string) (*model.Lyrics, error) {
	song, err := s.store.GetSong(ctx, songID)
	if err != nil {
		return nil, err
	}
	lyr := &model.Lyrics{Lines: []model.LyricsLine{}}
	if strings.TrimSpace(song.Lyrics) != "" {
		for _, line := range strings.Split(song.Lyrics, "\n") {
			lyr.Lines = append(lyr.Lines, model.LyricsLine{Text: line})
		}
		return lyr, nil
	}

	lrcKey := replaceExt(song.Path, ".lrc")
	ok, err := s.st.ObjectExists(ctx, lrcKey)
	if err != nil {
		return nil, err
	}
	if !ok {
		return lyr, nil
	}
	obj, err := s.st.Open(ctx, lrcKey, 0, -1)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(obj)
	obj.Close()
	if err != nil {
		return nil, err
	}
	lines := ParseLRC(data)
	if len(lines) > 0 {
		lyr.Synced = true
		lyr.Lines = lines
	}
	return lyr, nil
}

// ParseLRC parses LRC (synced lyrics) content into lines with timestamps in
// milliseconds. Lines without a timestamp are kept with Start = nil.
func ParseLRC(data []byte) []model.LyricsLine {
	text := strings.TrimSpace(strings.ReplaceAll(string(data), "\r\n", "\n"))
	if text == "" {
		return nil
	}
	var out []model.LyricsLine
	var offset int64
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if m := lrcOffsetRe.FindStringSubmatch(line); m != nil {
			offset, _ = strconv.ParseInt(m[1], 10, 64)
			continue
		}
		matches := lrcTimeRe.FindAllStringSubmatchIndex(line, -1)
		if len(matches) == 0 {
			// plain line: skip metadata tags ([ti:], [ar:], ...) but keep text
			if strings.HasPrefix(line, "[") && strings.Contains(line, "]") {
				continue
			}
			out = append(out, model.LyricsLine{Text: line})
			continue
		}
		textStart := matches[len(matches)-1][1]
		txt := strings.TrimSpace(line[textStart:])
		if txt == "" {
			continue
		}
		for _, m := range matches {
			mm := line[m[0]+1 : m[1]-1]
			if ts, ok := parseLRCTime(mm); ok {
				ts += offset
				if ts < 0 {
					ts = 0
				}
				out = append(out, model.LyricsLine{Text: txt, Start: &ts})
			}
		}
	}
	return out
}

func parseLRCTime(s string) (int64, bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, false
	}
	min, err1 := strconv.Atoi(parts[0])
	if err1 != nil || min < 0 {
		return 0, false
	}
	secPart := parts[1]
	sec := 0
	fraction := 0
	if dot := strings.IndexAny(secPart, "."); dot >= 0 {
		fraction, _ = strconv.Atoi(secPart[dot+1:])
		sec, _ = strconv.Atoi(secPart[:dot])
	} else {
		sec, _ = strconv.Atoi(secPart)
	}
	if sec < 0 || sec >= 60 {
		return 0, false
	}
	// fraction digits: 1-3 digits interpreted as milliseconds-ish
	fracMs := fraction
	switch {
	case fraction >= 1000:
	case fraction >= 100:
		fracMs = fraction
	case fraction >= 10:
		fracMs = fraction * 10
	default:
		fracMs = fraction * 100
	}
	return int64(min)*60000 + int64(sec)*1000 + int64(fracMs), true
}

func replaceExt(path, newExt string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return path + newExt
	}
	return path[:len(path)-len(ext)] + newExt
}
