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

	"github.com/minio/minio-go/v7"

	"play-music/internal/artwork"
	"play-music/internal/config"
	"play-music/internal/metadata"
	"play-music/internal/model"
	"play-music/internal/storage"
	"play-music/internal/store"
	"play-music/internal/stream"
)

var coverNames = map[string]bool{
	"cover.jpg": true, "cover.jpeg": true, "cover.png": true, "cover.webp": true,
	"folder.jpg": true, "folder.jpeg": true, "folder.png": true, "folder.webp": true,
	"front.jpg": true, "front.jpeg": true, "front.png": true,
	"album.jpg": true, "album.jpeg": true, "album.png": true,
	"art.jpg": true, "art.png": true, "artist.jpg": true, "artist.png": true,
}

// scanTempPrefix names the scratch copy of a file being indexed. A tag written
// by a previous interrupted run can carry the prefix, so the title fallback
// tests for it too.
const scanTempPrefix = "pm-scan-"

type Scanner struct {
	cfg   *config.Config
	st    *storage.Storage
	store *store.Store
	art   *artwork.Service
	log   *slog.Logger
	mu    sync.Mutex
}

func New(cfg *config.Config, st *storage.Storage, st2 *store.Store, art *artwork.Service, log *slog.Logger) *Scanner {
	return &Scanner{cfg: cfg, st: st, store: st2, art: art, log: log}
}

type audioObj struct {
	key   string
	size  int64
	mtime time.Time
}

// Run scans the bucket once. A scan never reports failures to a caller: both
// callers are unconditional background runs, so every failure is logged here.
func (s *Scanner) Run(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()
	s.log.Info("scan started")

	existing, err := s.store.LoadSongPaths(ctx)
	if err != nil {
		s.log.Error("load song paths", "err", err)
		return
	}

	// Do not process an incomplete inventory or delete from it.
	audios, coverByFolder, m3uFiles, seenPaths, err := collectInventory(ctx, s.st.List(ctx))
	if err != nil {
		s.log.Error("list error", "err", err)
		return
	}

	const workers = 4
	jobs := make(chan audioObj)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var added, updated, skipped int
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for a := range jobs {
				res := s.processAudio(ctx, a, existing[a.key])
				mu.Lock()
				added += res.added
				updated += res.updated
				skipped += res.skipped
				mu.Unlock()
			}
		}()
	}
	for _, a := range audios {
		jobs <- a
	}
	close(jobs)
	wg.Wait()

	if err := s.importFolderCovers(ctx, coverByFolder); err != nil {
		s.log.Error("folder covers failed", "err", err)
	}

	var deleted int
	if s.cfg.ScannerDeleteMissing {
		keep := make([]string, 0, len(seenPaths))
		for k := range seenPaths {
			keep = append(keep, k)
		}
		if n, err := s.store.DeleteMissing(ctx, keep); err != nil {
			s.log.Error("delete missing failed", "err", err)
		} else {
			deleted = int(n)
		}
	}

	// Imported playlists belong to the admin, so they cannot be created without
	// one; without an admin owner the import is skipped rather than attempted
	// with an empty user id.
	adminID, err := s.store.GetAdminID(ctx)
	if err != nil {
		s.log.Warn("no admin user for playlist import", "err", err)
	} else {
		for _, m3u := range m3uFiles {
			if err := s.importM3U(ctx, m3u, adminID); err != nil {
				s.log.Warn("m3u import failed", "file", m3u, "err", err)
			}
		}
	}

	s.log.Info("scan finished",
		"added", added, "updated", updated, "skipped", skipped,
		"deleted", deleted, "duration", time.Since(start).Round(time.Millisecond).String())
}

func collectInventory(ctx context.Context, objects <-chan minio.ObjectInfo) ([]audioObj, map[string]string, []string, map[string]bool, error) {
	var audios []audioObj
	covers := map[string]string{}
	var playlists []string
	seen := map[string]bool{}
	for obj := range objects {
		if obj.Err != nil {
			return nil, nil, nil, nil, obj.Err
		}
		if obj.Size == 0 {
			continue
		}
		key := obj.Key
		seen[key] = true
		lower := strings.ToLower(key)
		switch {
		case stream.IsAudioExt(filepath.Ext(lower)):
			audios = append(audios, audioObj{key, obj.Size, obj.LastModified})
		case strings.HasSuffix(lower, ".m3u"):
			playlists = append(playlists, key)
		case coverNames[filepath.Base(lower)]:
			folder := store.FolderOf(key)
			if _, ok := covers[folder]; !ok {
				covers[folder] = key
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, nil, nil, err
	}
	return audios, covers, playlists, seen, nil
}

type audioResult struct{ added, updated, skipped int }

func (s *Scanner) processAudio(ctx context.Context, a audioObj, prev store.SongFileInfo) audioResult {
	key, size, mtime := a.key, a.size, a.mtime

	if prev.Size == size && mtimeEqual(prev.Mtime, mtime) {
		return audioResult{skipped: 1}
	}
	if _, err := s.indexAudio(ctx, a, filepath.Base(key)); err != nil {
		s.log.Error("audio index failed", "key", key, "err", err)
		return audioResult{}
	}
	if prev.Size == 0 && prev.Mtime.IsZero() {
		return audioResult{added: 1}
	}
	return audioResult{updated: 1}
}

func (s *Scanner) IndexFile(ctx context.Context, key, displayName string) (*model.Song, error) {
	info, err := s.st.Stat(ctx, key)
	if err != nil {
		return nil, err
	}
	return s.indexAudio(ctx, audioObj{key: key, size: info.Size, mtime: info.LastModified}, displayName)
}

func (s *Scanner) indexAudio(ctx context.Context, a audioObj, displayName string) (*model.Song, error) {
	key, size, mtime := a.key, a.size, a.mtime

	tmp, err := os.CreateTemp("", scanTempPrefix+"*"+filepath.Ext(key))
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

	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(key)), ".")
	tags, err := metadata.ReadAudio(tmp.Name(), size, s.cfg.FfmpegPath)
	if err != nil {
		return nil, err
	}
	if displayName == "" {
		displayName = key
	}
	if title := strings.TrimSpace(tags.Title); title == "" || strings.HasPrefix(title, scanTempPrefix) {
		tags.Title = strings.TrimSuffix(filepath.Base(displayName), filepath.Ext(displayName))
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
	}
	if song.Album == "" {
		song.Album = store.UnknownArtist
	}

	albumArtist := tags.AlbumArtist
	if albumArtist == "" {
		albumArtist = tags.Artist
	}
	if song.Artist == "" && albumArtist != "" {
		song.Artist = albumArtist
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
	songArtistID := artistID
	if tags.Artist != "" && tags.Artist != albumArtist {
		if id, err := s.store.GetOrCreateArtist(ctx, tags.Artist); err == nil {
			songArtistID = id
		} else {
			s.log.Error("song artist upsert", "err", err)
		}
	}
	song.ArtistID = songArtistID
	song.AlbumID = albumID

	song.HasCover = tags.Picture != nil
	id, err := s.store.UpsertSong(ctx, song, mtime, size)
	if err != nil {
		return nil, err
	}
	song.ID = id

	// Genres become categories automatically: each song with a genre tag is
	// assigned to a category named after it (created on demand). IgnoredGenre is
	// skipped because migration 0008 deleted that category for good.
	if genre := strings.TrimSpace(song.Genre); genre != "" && !strings.EqualFold(genre, store.IgnoredGenre) {
		if catID, cerr := s.store.GetOrCreateCategory(ctx, song.Genre); cerr != nil {
			s.log.Warn("category from genre", "genre", song.Genre, "err", cerr)
		} else if catID != "" {
			if aerr := s.store.AddSongToCategory(ctx, catID, song.ID); aerr != nil {
				s.log.Warn("assign song to category", "genre", song.Genre, "err", aerr)
			}
		}
	}

	if tags.Picture != nil && len(tags.Picture.Data) > 0 {
		if err := s.art.Put(ctx, store.EntitySong, song.ID, tags.Picture.Data); err != nil {
			s.log.Warn("art upsert", "song", song.ID, "err", err)
		}
		// Keep the album-level cover as a fallback for songs without art.
		if err := s.art.Put(ctx, store.EntityAlbum, albumID, tags.Picture.Data); err != nil {
			s.log.Warn("art upsert", "album", albumID, "err", err)
		}
	}
	return song, nil
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
		if err := s.art.Put(ctx, store.EntityAlbum, albumID, data); err != nil {
			s.log.Warn("folder cover upsert", "album", albumID, "err", err)
		}
	}
	return nil
}

func (s *Scanner) importM3U(ctx context.Context, key, adminID string) error {
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
	folder := store.FolderOf(key)

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
	if err := s.store.ImportPlaylist(ctx, adminID, name, "admin", paths); err != nil {
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

func mtimeEqual(a, b time.Time) bool {
	if a.IsZero() || b.IsZero() {
		return false
	}
	return a.Equal(b) || a.Sub(b) < time.Second && b.Sub(a) < time.Second
}
