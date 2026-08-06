package scanner

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"play-music/internal/config"
	"play-music/internal/metadata"
	"play-music/internal/model"
	"play-music/internal/storage"
	"play-music/internal/store"
)

var audioExts = map[string]bool{
	".mp3": true, ".flac": true, ".m4a": true, ".aac": true, ".ogg": true,
	".opus": true, ".wav": true, ".wma": true, ".aiff": true, ".aif": true,
	".wv": true, ".tak": true, ".ape": true,
}

var coverNames = map[string]bool{
	"cover.jpg": true, "cover.jpeg": true, "cover.png": true, "cover.webp": true,
	"folder.jpg": true, "folder.jpeg": true, "folder.png": true, "folder.webp": true,
	"front.jpg": true, "front.jpeg": true, "front.png": true,
	"album.jpg": true, "album.jpeg": true, "album.png": true,
	"art.jpg": true, "art.png": true, "artist.jpg": true, "artist.png": true,
}

type Scanner struct {
	cfg   *config.Config
	st    *storage.Storage
	store *store.Store
	log   *slog.Logger
	mu    sync.Mutex
}

func New(cfg *config.Config, st *storage.Storage, st2 *store.Store, log *slog.Logger) *Scanner {
	return &Scanner{cfg: cfg, st: st, store: st2, log: log}
}

type Result struct {
	Added   int
	Updated int
	Skipped int
	Deleted int
	Error   error
}

type audioObj struct {
	key   string
	size  int64
	mtime time.Time
}

// Run performs a full library scan: audio indexing, embedded/folder artwork
// and .m3u playlist import.
func (s *Scanner) Run(ctx context.Context) Result {
	s.mu.Lock()
	defer s.mu.Unlock()

	res := Result{}
	start := time.Now()
	s.log.Info("scan started")

	existing, err := s.store.LoadSongPaths(ctx)
	if err != nil {
		res.Error = err
		return res
	}

	// Pass 1: list the bucket and classify objects.
	var audios []audioObj
	// folder -> cover key
	coverByFolder := map[string]string{}
	var m3uFiles []string
	seenPaths := map[string]bool{}

	for obj := range s.st.List(ctx) {
		if obj.Err != nil {
			s.log.Error("list error", "err", obj.Err)
			continue
		}
		if obj.Size == 0 {
			continue
		}
		key := obj.Key
		seenPaths[key] = true
		lower := strings.ToLower(key)
		ext := filepath.Ext(lower)
		switch {
		case audioExts[ext]:
			audios = append(audios, audioObj{key, obj.Size, obj.LastModified})
		case strings.HasSuffix(lower, ".m3u") || strings.HasSuffix(lower, ".m3u8"):
			m3uFiles = append(m3uFiles, key)
		case coverNames[filepath.Base(lower)]:
			folder := folderOf(key)
			if _, ok := coverByFolder[folder]; !ok {
				coverByFolder[folder] = key
			}
		}
	}

	// Pass 2: process audio files (changed ones) with a small worker pool.
	const workers = 4
	jobs := make(chan audioObj)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for a := range jobs {
				res2 := s.processAudio(ctx, a, existing[a.key])
				mu.Lock()
				res.Added += res2.added
				res.Updated += res2.updated
				res.Skipped += res2.skipped
				mu.Unlock()
			}
		}()
	}
	for _, a := range audios {
		jobs <- a
	}
	close(jobs)
	wg.Wait()

	// Pass 3: folder covers for albums still without artwork.
	if err := s.importFolderCovers(ctx, coverByFolder); err != nil {
		s.log.Error("folder covers failed", "err", err)
	}

	// Pass 4: remove songs no longer present.
	keep := make([]string, 0, len(seenPaths))
	for k := range seenPaths {
		keep = append(keep, k)
	}
	if deleted, err := s.store.DeleteMissing(ctx, keep); err != nil {
		s.log.Error("delete missing failed", "err", err)
	} else {
		res.Deleted = int(deleted)
	}

	// Pass 5: import playlists.
	for _, m3u := range m3uFiles {
		if err := s.importM3U(ctx, m3u); err != nil {
			s.log.Warn("m3u import failed", "file", m3u, "err", err)
		}
	}

	_ = s.store.SetSetting(ctx, "last_scan_at", time.Now().UTC().Format(time.RFC3339))
	s.log.Info("scan finished",
		"added", res.Added, "updated", res.Updated, "skipped", res.Skipped,
		"deleted", res.Deleted, "duration", time.Since(start).Round(time.Millisecond).String())
	return res
}

type audioResult struct{ added, updated, skipped int }

func (s *Scanner) processAudio(ctx context.Context, a audioObj, prev store.SongFileInfo) audioResult {
	key, size, mtime := a.key, a.size, a.mtime

	// Skip unchanged files.
	if prev.Size == size && mtimeEqual(prev.Mtime, mtime) {
		return audioResult{skipped: 1}
	}

	tmp, err := os.CreateTemp("", "pm-scan-*"+filepath.Ext(key))
	if err != nil {
		s.log.Error("temp file", "key", key, "err", err)
		return audioResult{}
	}
	defer os.Remove(tmp.Name())

	obj, err := s.st.Open(ctx, key, 0, -1)
	if err != nil {
		s.log.Error("download failed", "key", key, "err", err)
		return audioResult{}
	}
	if _, err := io.Copy(tmp, obj); err != nil {
		obj.Close()
		s.log.Error("download failed", "key", key, "err", err)
		return audioResult{}
	}
	obj.Close()
	tmp.Close()

	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(key)), ".")
	tags, err := metadata.Read(tmp.Name(), size)
	if err != nil {
		s.log.Debug("metadata read failed", "key", key, "err", err)
	}

	song := &model.Song{
		ID:          store.NewID(),
		Path:        key,
		Title:       tags.Title,
		Artist:      tags.Artist,
		Album:       tags.Album,
		Year:        tags.Year,
		Genre:       tags.Genre,
		Duration:    tags.Duration,
		Format:      ext,
		Bitrate:     tags.Bitrate,
		SampleRate:  tags.SampleRate,
		TrackNumber: tags.Track,
		DiscNumber:  tags.Disc,
		Lyrics:      tags.Lyrics,
	}
	if song.Album == "" {
		song.Album = "Desconhecido"
	}

	albumArtist := tags.AlbumArtist
	if albumArtist == "" {
		albumArtist = tags.Artist
	}
	artistID := ""
	if albumArtist != "" {
		artistID, err = s.store.GetOrCreateArtist(ctx, albumArtist)
		if err != nil {
			s.log.Error("artist upsert", "name", albumArtist, "err", err)
		}
	}
	albumID, err := s.store.GetOrCreateAlbum(ctx, song.Album, albumArtist, artistID, tags.Year)
	if err != nil {
		s.log.Error("album upsert", "name", song.Album, "err", err)
	}
	song.ArtistID = artistID
	song.AlbumID = albumID

	song.HasCover = tags.Picture != nil
	id, err := s.store.UpsertSong(ctx, song, mtime, size)
	if err != nil {
		s.log.Error("song upsert", "key", key, "err", err)
		return audioResult{}
	}
	song.ID = id

	if tags.Picture != nil && len(tags.Picture.Data) > 0 {
		mime := tags.Picture.MIMEType
		if mime == "" {
			mime = "image/jpeg"
		}
		if err := s.store.UpsertArt(ctx, "album", albumID, tags.Picture.Data, mime); err != nil {
			s.log.Warn("art upsert", "album", albumID, "err", err)
		}
	}

	if prev.Size == 0 && prev.Mtime.IsZero() {
		return audioResult{added: 1}
	}
	return audioResult{updated: 1}
}

func (s *Scanner) importFolderCovers(ctx context.Context, coverByFolder map[string]string) error {
	ids, err := s.store.AlbumsWithoutArt(ctx)
	if err != nil {
		return err
	}
	for _, albumID := range ids {
		folder, err := s.store.AlbumFolder(ctx, albumID)
		if err != nil || folder == "" {
			continue
		}
		cover, ok := coverByFolder[folder]
		if !ok {
			continue
		}
		obj, err := s.st.Open(ctx, cover, 0, -1)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(obj)
		obj.Close()
		if err != nil || len(data) == 0 {
			continue
		}
		mime := mimeForExt(filepath.Ext(cover))
		if err := s.store.UpsertArt(ctx, "album", albumID, data, mime); err != nil {
			s.log.Warn("folder cover upsert", "album", albumID, "err", err)
		}
	}
	return nil
}

// importM3U parses an .m3u file and (re)creates the matching playlist.
func (s *Scanner) importM3U(ctx context.Context, key string) error {
	base := filepath.Base(key)
	if strings.HasPrefix(base, ".") {
		return nil
	}
	obj, err := s.st.Open(ctx, key, 0, -1)
	if err != nil {
		return err
	}
	defer obj.Close()
	data, err := io.ReadAll(obj)
	if err != nil {
		return err
	}
	text := decodePlaylistText(data)
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return nil
	}
	name := strings.TrimSuffix(base, filepath.Ext(base))
	folder := folderOf(key)

	var paths []string
	for _, raw := range lines {
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entry := strings.ReplaceAll(line, "\\", "/")
		entry = strings.TrimPrefix(entry, "/")
		if folder != "" && !strings.HasPrefix(entry, folder) {
			entry = folder + entry
		}
		paths = append(paths, entry)
	}
	if err := s.store.ImportPlaylist(ctx, name, "admin", paths); err != nil {
		return err
	}
	s.log.Info("playlist imported", "name", name, "tracks", len(paths))
	return nil
}

func decodePlaylistText(data []byte) string {
	// BOM detection: UTF-8, UTF-16 LE/BE.
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return string(data[3:])
	}
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		return decodeUTF16(data[2:], true)
	}
	if len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF {
		return decodeUTF16(data[2:], false)
	}
	return string(data)
}

func decodeUTF16(b []byte, littleEndian bool) string {
	if len(b)%2 == 1 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		if littleEndian {
			u = append(u, uint16(b[i])|uint16(b[i+1])<<8)
		} else {
			u = append(u, uint16(b[i])<<8|uint16(b[i+1]))
		}
	}
	return strings.ToValidUTF8(string(runesFromUint16(u)), "")
}

func runesFromUint16(u []uint16) []rune {
	r := make([]rune, len(u))
	for i, v := range u {
		r[i] = rune(v)
	}
	return r
}

func mimeForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "image/jpeg"
	}
}

func folderOf(key string) string {
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == '/' {
			return key[:i+1]
		}
	}
	return ""
}

func mtimeEqual(a, b time.Time) bool {
	if a.IsZero() || b.IsZero() {
		return false
	}
	return a.Equal(b) || a.Sub(b) < time.Second && b.Sub(a) < time.Second
}
