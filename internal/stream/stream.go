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

	cdnMu      sync.Mutex
	cdnRangeOK bool
	cdnChecked time.Time
}

type CacheDirs struct {
	Transcode string
	Stream    string
}

func New(cfg *config.Config, st *storage.Storage, log *slog.Logger) (*Service, error) {
	dirs := CacheDirs{
		Transcode: "var/cache/transcode",
		Stream:    "var/cache/stream",
	}
	for _, d := range []string{dirs.Transcode, dirs.Stream} {
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

// cdnProbeTTL controls how often the CDN range capability is re-checked.
const cdnProbeTTL = 2 * time.Minute

// CDNRangeOK reports whether the Bunny CDN pull zone answers Range requests
// on cache misses (requires "Optimize for large object delivery" enabled on
// the zone). The probe result is cached briefly so the app automatically
// falls back to the local proxy until the zone handles ranges.
func (s *Service) CDNRangeOK(ctx context.Context, path string) bool {
	if !s.cfg.CDNEnabled || s.cfg.CDNBaseURL == "" || s.cfg.CDNTokenKey == "" {
		return false
	}
	s.cdnMu.Lock()
	defer s.cdnMu.Unlock()
	if time.Since(s.cdnChecked) < cdnProbeTTL {
		return s.cdnRangeOK
	}
	s.cdnChecked = time.Now()
	s.cdnRangeOK = s.probeCDNRange(path)
	return s.cdnRangeOK
}

// probeCDNRange issues a tiny Range request against a fresh signed URL. A
// fresh URL is almost always a cache miss, so a 206 means the zone processes
// ranges for uncached content. The body is read for one byte and aborted, so
// a zone that ignores Range (streaming the whole file) is never downloaded.
func (s *Service) probeCDNRange(path string) bool {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.CDNURL(path), nil)
	if err != nil {
		return false
	}
	req.Header.Set("Range", "bytes=1-2")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	buf := make([]byte, 1)
	_, _ = io.ReadFull(resp.Body, buf)
	return resp.StatusCode == http.StatusPartialContent
}

// mimeByFormat maps a stored song format to its Content-Type for ServeNative.
var mimeByFormat = map[string]string{
	"mp3": "audio/mpeg", "m4a": "audio/mp4", "aac": "audio/aac",
	"ogg": "audio/ogg", "opus": "audio/ogg", "wav": "audio/wav",
	"flac": "audio/flac",
}

// videoMimeByFormat maps a karaoke video format to its Content-Type.
var videoMimeByFormat = map[string]string{
	"mp4": "video/mp4", "m4v": "video/mp4", "webm": "video/webm",
	"mkv": "video/x-matroska",
}

// mediaFile is the minimal object info the stream cache needs. Both songs and
// karaokes convert to it, so audio and video share the same caching/ranging
// pipeline without touching the song model.
type mediaFile struct {
	ID        string
	Path      string
	Size      int64
	UpdatedAt time.Time
}

func mediaFileFromSong(song *model.Song) mediaFile {
	return mediaFile{ID: song.ID, Path: song.Path, Size: song.Size, UpdatedAt: song.UpdatedAt}
}

func mediaFileFromKaraoke(k *model.Karaoke) mediaFile {
	return mediaFile{ID: k.ID, Path: k.Path, Size: k.Size, UpdatedAt: k.UpdatedAt}
}

// streamChunkSize caps each response so the browser downloads the file piece
// by piece instead of pulling the whole track in one request. 5 MiB matches
// the Bunny CDN "large object" chunk size, keeps responses small (friendly to
// proxies/CDNs with response limits) and bounds per-request work.
const streamChunkSize = 5 << 20 // 5 MiB

// ServeNative streams a native-format song through w with full HTTP Range
// support (206 responses).
//
// The MinIO endpoint (behind its caching proxy) ignores Range requests and
// always answers 200 with the whole object, so ranged streaming from the
// bucket is not possible. The file is therefore cached once to disk (like the
// transcode cache) and served locally with Range + the streamChunkSize cap,
// which makes <audio> progressively fetch the file in small pieces (CDN-safe)
// and seeking work even though the Bunny CDN also drops Range on cache miss.
func (s *Service) ServeNative(ctx context.Context, w http.ResponseWriter, r *http.Request, song *model.Song) error {
	media := mediaFileFromSong(song)
	cacheFile := filepath.Join(s.dirs.Stream, media.ID+filepath.Ext(media.Path))
	if err := s.ensureStreamCache(ctx, media, cacheFile); err != nil {
		return err
	}
	if mime := mimeByFormat[song.Format]; mime != "" {
		w.Header().Set("Content-Type", mime)
	}
	w.Header().Set("Accept-Ranges", "bytes")

	// Progressive chunking: clamp a large/open-ended range to chunkSize so
	// each response is bounded and the browser asks for the next chunk when
	// its buffer runs low.
	if rng := r.Header.Get("Range"); rng != "" {
		if start, end, ok := parseRange(rng, mediaSize(media, cacheFile)); ok && end-start+1 > streamChunkSize {
			r.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, start+streamChunkSize-1))
		}
	}
	http.ServeFile(w, r, cacheFile)
	return nil
}

// ServeVideo streams a karaoke video through w with full HTTP Range support,
// reusing the same disk cache + progressive chunking pipeline as audio.
func (s *Service) ServeVideo(ctx context.Context, w http.ResponseWriter, r *http.Request, k *model.Karaoke) error {
	media := mediaFileFromKaraoke(k)
	cacheFile := filepath.Join(s.dirs.Stream, media.ID+filepath.Ext(media.Path))
	if err := s.ensureStreamCache(ctx, media, cacheFile); err != nil {
		return err
	}
	if mime := videoMimeByFormat[k.Format]; mime != "" {
		w.Header().Set("Content-Type", mime)
	}
	w.Header().Set("Accept-Ranges", "bytes")

	if rng := r.Header.Get("Range"); rng != "" {
		if start, end, ok := parseRange(rng, mediaSize(media, cacheFile)); ok && end-start+1 > streamChunkSize {
			r.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, start+streamChunkSize-1))
		}
	}
	http.ServeFile(w, r, cacheFile)
	return nil
}

func mediaSize(media mediaFile, cacheFile string) int64 {
	if fi, err := os.Stat(cacheFile); err == nil {
		return fi.Size()
	}
	return media.Size
}

// ensureStreamCache downloads the object to the local stream cache when stale
// or missing. The MinIO endpoint ignores Range requests, so a full download
// is required before the file can be served locally with ranges.
func (s *Service) ensureStreamCache(ctx context.Context, media mediaFile, cacheFile string) error {
	if info, err := os.Stat(cacheFile); err == nil {
		if s.fresh(info.ModTime(), media.UpdatedAt) {
			return nil
		}
	}

	mu := s.flight(media.ID)
	mu.Lock()
	defer mu.Unlock()

	if info, err := os.Stat(cacheFile); err == nil {
		if s.fresh(info.ModTime(), media.UpdatedAt) {
			return nil
		}
	}

	tmp, err := os.CreateTemp(s.dirs.Stream, "s-*"+filepath.Ext(media.Path))
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	src, err := s.st.Open(ctx, media.Path, 0, -1)
	if err != nil {
		tmp.Close()
		return err
	}
	_, copyErr := io.Copy(tmp, src)
	src.Close()
	if err := tmp.Close(); err != nil && copyErr == nil {
		copyErr = err
	}
	if copyErr != nil {
		return copyErr
	}
	if err := os.Rename(tmpName, cacheFile); err != nil {
		s.log.Warn("stream cache rename failed", "err", err)
		cacheFile = tmpName
	}
	s.enforceCacheLimit(s.dirs.Stream, s.cfg.TranscodingCacheSize)
	return nil
}

// parseRange parses a single-range "bytes=" header (RFC 7233). It returns
// false for multi-range or malformed specs, leaving the header to ServeFile
// (multi-range is never sent by media elements).
func parseRange(rng string, size int64) (start, end int64, ok bool) {
	if !strings.HasPrefix(rng, "bytes=") || strings.Contains(rng, ",") {
		return 0, 0, false
	}
	spec := strings.TrimPrefix(rng, "bytes=")
	dash := strings.IndexByte(spec, '-')
	if dash < 0 {
		return 0, 0, false
	}
	startStr, endStr := strings.TrimSpace(spec[:dash]), strings.TrimSpace(spec[dash+1:])
	if startStr == "" {
		// Suffix range: bytes=-N (last N bytes).
		n, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, false
		}
		if n > size {
			n = size
		}
		return size - n, size - 1, true
	}
	start, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, false
	}
	if endStr == "" {
		return start, size - 1, true
	}
	end, err = strconv.ParseInt(endStr, 10, 64)
	if err != nil || end < start {
		return 0, 0, false
	}
	if end >= size {
		end = size - 1
	}
	return start, end, true
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
