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
	"play-music/internal/model"
	"play-music/internal/storage"
)

// NativeFormats are served directly (redirect); anything else is transcoded.
var NativeFormats = map[string]bool{
	"mp3": true, "m4a": true, "aac": true, "ogg": true, "opus": true,
	"wav": true, "flac": true,
}

type Service struct {
	cfg     *config.Config
	st      *storage.Storage
	log     *slog.Logger
	dirs    CacheDirs
	mu      sync.Mutex
	flights map[string]*sync.Mutex
}

type CacheDirs struct {
	Transcode string
}

func New(cfg *config.Config, st *storage.Storage, log *slog.Logger) (*Service, error) {
	dirs := CacheDirs{
		Transcode: "var/cache/transcode",
	}
	for _, d := range []string{dirs.Transcode} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	return &Service{
		cfg:     cfg,
		st:      st,
		log:     log,
		dirs:    dirs,
		flights: make(map[string]*sync.Mutex),
	}, nil
}

// StreamURL returns a time-limited URL for the configured S3-compatible
// storage. Access is checked by the handler before a URL is issued.
func (s *Service) StreamURL(ctx context.Context, song *model.Song) (string, error) {
	return s.st.PresignedURL(ctx, song.Path, s.cfg.StreamURLTTL)
}

// Transcode writes an mp3 version of the song to w, using the on-disk cache
// keyed by song id + file mtime.
func (s *Service) Transcode(ctx context.Context, w http.ResponseWriter, r *http.Request, song *model.Song) error {
	ffmpeg := s.ffmpegPath()
	if ffmpeg == "" {
		return fmt.Errorf("ffmpeg não encontrado; transcodificação indisponível")
	}

	cacheFile := filepath.Join(s.dirs.Transcode, song.ID+"-mp3.mp3")
	if info, err := os.Stat(cacheFile); err == nil {
		if s.fresh(info.ModTime(), song.UpdatedAt) {
			http.ServeFile(w, r, cacheFile)
			return nil
		}
	}

	mu := s.flight(song.ID)
	mu.Lock()
	defer mu.Unlock()

	if info, err := os.Stat(cacheFile); err == nil {
		if s.fresh(info.ModTime(), song.UpdatedAt) {
			http.ServeFile(w, r, cacheFile)
			return nil
		}
	}

	tmp, err := os.CreateTemp(s.dirs.Transcode, "t-*.mp3")
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
		"-vn", "-acodec", "libmp3lame", "-b:a", "192k", "-f", "mp3",
		"-y", tmpName)
	cmd.Stdin = src
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("transcodificação falhou: %w", err)
	}
	if err := os.Rename(tmpName, cacheFile); err != nil {
		s.log.Warn("transcode rename failed", "err", err)
		cacheFile = tmpName
	}
	s.enforceCacheLimit(s.dirs.Transcode, s.cfg.TranscodingCacheSize)
	http.ServeFile(w, r, cacheFile)
	return nil
}

func (s *Service) fresh(fileMtime, songUpdated time.Time) bool {
	return fileMtime.After(songUpdated.Add(-2 * time.Minute))
}

func (s *Service) ffmpegPath() string {
	if s.cfg.FfmpegPath != "" {
		return s.cfg.FfmpegPath
	}
	p, err := exec.LookPath("ffmpeg")
	if err != nil {
		return ""
	}
	return p
}

func (s *Service) flight(key string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.flights[key]
	if !ok {
		m = &sync.Mutex{}
		s.flights[key] = m
	}
	return m
}

// enforceCacheLimit removes oldest files once the dir exceeds the limit.
func (s *Service) enforceCacheLimit(dir string, limit int64) {
	if limit <= 0 {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var total int64
	type f struct {
		path string
		mod  time.Time
	}
	var files []f
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		total += info.Size()
		files = append(files, f{filepath.Join(dir, e.Name()), info.ModTime()})
	}
	if total <= limit {
		return
	}
	for i := 1; i < len(files); i++ {
		for j := i; j > 0 && files[j].mod.Before(files[j-1].mod); j-- {
			files[j], files[j-1] = files[j-1], files[j]
		}
	}
	for _, fi := range files {
		if total <= limit {
			break
		}
		info, err := os.Stat(fi.path)
		if err == nil {
			total -= info.Size()
		}
		os.Remove(fi.path)
	}
}
