package scanner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"play-music/internal/metadata"
	"play-music/internal/model"
	"play-music/internal/store"
)

// ErrInvalidVideo marks files whose metadata cannot be read or that report no
// duration — not actually decodable video (junk renamed to .mp4, truncated).
// Uploads map it to a 400.
var ErrInvalidVideo = errors.New("arquivo de vídeo inválido ou corrompido")

// videoExts are the karaoke formats accepted by the upload endpoint.
var videoExts = map[string]bool{
	".mp4": true, ".m4v": true, ".webm": true, ".mkv": true,
}

// IndexKaraoke downloads and indexes a single video object, returning the
// karaoke record (used right after an admin upload so it is playable
// immediately). A thumbnail is extracted with ffmpeg on a best-effort basis:
// when ffmpeg is unavailable the karaoke is still indexed (artwork falls back
// to the placeholder until a manual photo is uploaded).
func (s *Scanner) IndexKaraoke(ctx context.Context, key, displayName string) (*model.Karaoke, error) {
	info, err := s.st.Stat(ctx, key)
	if err != nil {
		return nil, err
	}

	tmp, err := os.CreateTemp("", "pm-karaoke-*"+filepath.Ext(key))
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())

	obj, err := s.st.Open(ctx, key, 0, -1)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(tmp, obj); err != nil {
		obj.Close()
		return nil, err
	}
	obj.Close()
	tmp.Close()

	ffprobe := metadata.ProbePath(ffmpegConfiguredPath())
	duration, _, _ := metadata.Probe(tmp.Name(), ffprobe)
	if duration <= 0 {
		return nil, fmt.Errorf("%w: duração não detectada (arquivo sem vídeo decodificável)", ErrInvalidVideo)
	}

	if displayName == "" {
		displayName = key
	}
	title := strings.TrimSuffix(filepath.Base(displayName), filepath.Ext(displayName))

	k := &model.Karaoke{
		ID:       store.NewID(),
		Path:     key,
		Title:    title,
		Duration: duration,
		Format:   strings.TrimPrefix(strings.ToLower(filepath.Ext(key)), "."),
	}

	id, err := s.store.UpsertKaraoke(ctx, k, info.LastModified, info.Size)
	if err != nil {
		return nil, err
	}
	k.ID = id

	// Thumbnail: best-effort, never fails the upload.
	thumbPath := tmp.Name() + ".jpg"
	if err := metadata.ExtractThumb(tmp.Name(), thumbPath); err != nil {
		s.log.Warn("karaoke thumbnail failed", "key", key, "err", err)
	} else {
		data, err := os.ReadFile(thumbPath)
		os.Remove(thumbPath)
		if err == nil && len(data) > 0 {
			if err := s.store.UpsertArt(ctx, "karaoke_thumb", id, data, "image/jpeg"); err != nil {
				s.log.Warn("karaoke thumb upsert", "karaoke", id, "err", err)
			}
		}
	}

	if fresh, err := s.store.GetKaraoke(ctx, id); err == nil {
		k = fresh
	}
	return k, nil
}

// ffmpegConfiguredPath lets the scanner locate ffprobe next to the configured
// ffmpeg binary (metadata.ProbePath resolves both cases).
func ffmpegConfiguredPath() string {
	return metadata.ConfiguredFFmpegPath()
}
