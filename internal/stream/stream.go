package stream

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"play-music/internal/config"
	"play-music/internal/fscache"
	"play-music/internal/metadata"
	"play-music/internal/model"
	"play-music/internal/storage"
)

// cacheSkew is how much older than the song row a transcode may be and still
// be served. The filesystem mtime and the database timestamp come from two
// clocks, and the row is written after the bytes, so a small negative window is
// expected on every fresh transcode.
const cacheSkew = 2 * time.Minute

type Service struct {
	cfg     *config.Config
	st      *storage.Storage
	log     *slog.Logger
	dir     string
	mu      sync.Mutex
	flights map[string]*flight
}

// flight is one in-progress transcode of a song. waiters counts the callers
// holding or awaiting it, so the entry can be dropped when the last leaves.
type flight struct {
	mu      sync.Mutex
	waiters int
}

func New(cfg *config.Config, st *storage.Storage, log *slog.Logger) (*Service, error) {
	dir := "var/cache/transcode"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Service{
		cfg:     cfg,
		st:      st,
		log:     log,
		dir:     dir,
		flights: make(map[string]*flight),
	}, nil
}

func (s *Service) StreamURL(ctx context.Context, song *model.Song) (string, error) {
	return s.st.PresignedURL(ctx, song.Path, s.cfg.StreamURLTTL)
}

func (s *Service) Transcode(ctx context.Context, w http.ResponseWriter, r *http.Request, song *model.Song) error {
	ffmpeg := metadata.FFmpegPath(s.cfg.FfmpegPath)
	if ffmpeg == "" {
		return fmt.Errorf("ffmpeg não encontrado; transcodificação indisponível")
	}

	cacheFile := filepath.Join(s.dir, song.ID+"-"+TranscodeFormat+"."+TranscodeFormat)
	if s.serveCached(w, r, cacheFile, song) {
		return nil
	}

	unlock := s.lockFlight(song.ID)
	defer unlock()

	if s.serveCached(w, r, cacheFile, song) {
		return nil
	}

	tmp, err := os.CreateTemp(s.dir, "t-*."+TranscodeFormat)
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpName)

	src, err := s.st.Open(ctx, song.Path, 0, -1)
	if err != nil {
		return err
	}
	defer src.Close()

	cmd := exec.CommandContext(ctx, ffmpeg,
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-i", "pipe:0",
		"-vn", "-acodec", TranscodeCodec, "-b:a", "192k", "-f", TranscodeFormat,
		"-y", tmpName)
	cmd.Stdin = src
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("transcodificação falhou: %w", err)
	}
	if err := os.Rename(tmpName, cacheFile); err != nil {
		s.log.Warn("transcode rename failed", "err", err)
		cacheFile = tmpName
	}
	fscache.EnforceLimit(s.dir, s.cfg.TranscodingCacheSize)
	http.ServeFile(w, r, cacheFile)
	return nil
}

func (s *Service) serveCached(w http.ResponseWriter, r *http.Request, cacheFile string, song *model.Song) bool {
	info, err := os.Stat(cacheFile)
	if err != nil || !info.ModTime().After(song.UpdatedAt.Add(-cacheSkew)) {
		return false
	}
	http.ServeFile(w, r, cacheFile)
	return true
}

// lockFlight serialises transcodes of one song without remembering a mutex per
// song forever: the entry is dropped as soon as the last waiter releases it, so
// the map only ever holds the songs currently being transcoded.
func (s *Service) lockFlight(key string) (unlock func()) {
	s.mu.Lock()
	f, ok := s.flights[key]
	if !ok {
		f = &flight{}
		s.flights[key] = f
	}
	f.waiters++
	s.mu.Unlock()

	f.mu.Lock()
	return func() {
		f.mu.Unlock()
		s.mu.Lock()
		if f.waiters--; f.waiters == 0 {
			delete(s.flights, key)
		}
		s.mu.Unlock()
	}
}
