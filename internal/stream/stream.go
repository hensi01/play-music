package stream

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	dirs := CacheDirs{Transcode: "var/cache/transcode"}
	if err := os.MkdirAll(dirs.Transcode, 0o755); err != nil {
		return nil, err
	}
	return &Service{
		cfg:     cfg,
		st:      st,
		log:     log,
		dirs:    dirs,
		flights: make(map[string]*sync.Mutex),
	}, nil
}

// CDNURL builds the signed URL for an object key, honoring the Bunny CDN
// token authentication mode configured in the environment.
//
// Important: the token must be hashed over the DECODED path while the URL
// carries the percent-ENCODED path (the CDN decodes the request path before
// validating the signature).
func (s *Service) CDNURL(key string) string {
	base := strings.TrimRight(s.cfg.CDNBaseURL, "/")
	prefix := strings.Trim(s.cfg.CDNPathPrefix, "/")
	var decodedPath string
	switch {
	case prefix == "":
		decodedPath = "/" + key
	default:
		decodedPath = "/" + prefix + "/" + key
	}
	encodedPath := escapePath(decodedPath)
	expires := strconv.FormatInt(time.Now().Add(s.cfg.CDNTokenTTL).Unix(), 10)
	token := s.sign(decodedPath, expires)
	return base + encodedPath + "?token=" + token + "&expires=" + expires
}

// sign computes the token over the DECODED path using the configured mode
// (Basic MD5 or Advanced HMAC-SHA256).
func (s *Service) sign(decodedPath, expires string) string {
	if s.cfg.CDNAdvancedAuth {
		// Advanced: token = "HS256-" + Base64URL(HMAC-SHA256(key, path+expires))
		mac := hmac.New(sha256.New, []byte(s.cfg.CDNTokenKey))
		mac.Write([]byte(decodedPath))
		mac.Write([]byte(expires))
		return "HS256-" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	}
	// Basic: token = Base64(MD5(key+path+expires)) with URL-safe alphabet.
	h := md5.New()
	h.Write([]byte(s.cfg.CDNTokenKey))
	h.Write([]byte(decodedPath))
	h.Write([]byte(expires))
	tok := base64.StdEncoding.EncodeToString(h.Sum(nil))
	tok = strings.ReplaceAll(tok, "+", "-")
	tok = strings.ReplaceAll(tok, "/", "_")
	return strings.TrimRight(tok, "=")
}

// escapePath percent-encodes each path segment (spaces and special chars).
func escapePath(p string) string {
	segments := strings.Split(p, "/")
	for i, seg := range segments {
		if seg != "" {
			segments[i] = url.PathEscape(seg)
		}
	}
	return strings.Join(segments, "/")
}

// StreamURL returns a signed, playable URL for a song (redirect target).
func (s *Service) StreamURL(ctx context.Context, song *model.Song) (string, error) {
	if s.cfg.CDNEnabled && s.cfg.CDNBaseURL != "" && s.cfg.CDNTokenKey != "" {
		return s.CDNURL(song.Path), nil
	}
	return s.st.PresignedURL(ctx, song.Path, 15*time.Minute)
}

// mimeByFormat maps a stored song format to its Content-Type for ServeNative.
var mimeByFormat = map[string]string{
	"mp3": "audio/mpeg", "m4a": "audio/mp4", "aac": "audio/aac",
	"ogg": "audio/ogg", "opus": "audio/ogg", "wav": "audio/wav",
	"flac": "audio/flac",
}

// ServeNative streams a native-format song through w with full HTTP Range
// support (206 responses), reading from the origin storage with ranged GETs.
//
// The Bunny CDN pull zone ignores Range requests for uncached content, which
// makes <audio> seekable ranges empty and silently breaks seeking; proxying
// the bytes through this server restores seeking regardless of the CDN.
func (s *Service) ServeNative(ctx context.Context, w http.ResponseWriter, r *http.Request, song *model.Song) error {
	info, err := s.st.Stat(ctx, song.Path)
	if err != nil {
		return err
	}
	if mime := mimeByFormat[song.Format]; mime != "" {
		w.Header().Set("Content-Type", mime)
	}
	w.Header().Set("Accept-Ranges", "bytes")
	reader := &objectSeeker{ctx: r.Context(), st: s.st, key: song.Path, size: info.Size}
	http.ServeContent(w, r, filepath.Base(song.Path), info.LastModified, reader)
	return nil
}

// objectSeeker is an io.ReadSeeker over an S3 object backed by ranged GETs.
// Each seek lazily opens a new ranged request, so ServeContent can seek
// around a large file without downloading it.
type objectSeeker struct {
	ctx    context.Context
	st     *storage.Storage
	key    string
	size   int64
	offset int64
	rc     io.ReadCloser
}

func (o *objectSeeker) Read(p []byte) (int, error) {
	if o.offset >= o.size {
		return 0, io.EOF
	}
	if o.rc == nil {
		rc, err := o.st.Open(o.ctx, o.key, o.offset, -1)
		if err != nil {
			return 0, err
		}
		o.rc = rc
	}
	n, err := o.rc.Read(p)
	o.offset += int64(n)
	if err == io.EOF {
		o.close()
	}
	return n, err
}

func (o *objectSeeker) Seek(offset int64, whence int) (int64, error) {
	var target int64
	switch whence {
	case io.SeekStart:
		target = offset
	case io.SeekCurrent:
		target = o.offset + offset
	case io.SeekEnd:
		target = o.size + offset
	default:
		return 0, fmt.Errorf("objectSeeker: invalid whence %d", whence)
	}
	if target < 0 {
		return 0, fmt.Errorf("objectSeeker: negative position %d", target)
	}
	o.close()
	o.offset = target
	return target, nil
}

func (o *objectSeeker) close() {
	if o.rc != nil {
		o.rc.Close()
		o.rc = nil
	}
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
