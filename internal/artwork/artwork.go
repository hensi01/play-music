package artwork

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/gif"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"play-music/internal/config"
	"play-music/internal/storage"
	"play-music/internal/store"
)

const (
	placeholderSize = 640
	artTTL          = 24 * time.Hour
)

type Service struct {
	cfg         *config.Config
	store       *store.Store
	stg         *storage.Storage
	log         *slog.Logger
	dir         string
	placeholder []byte
	cache       cache
}

type cache interface {
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, data []byte)
}

func New(cfg *config.Config, st *store.Store, stg *storage.Storage, log *slog.Logger) (*Service, error) {
	dir := "var/cache/art"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Service{cfg: cfg, store: st, stg: stg, log: log, dir: dir}
	s.placeholder = s.makePlaceholder()
	limit := cfg.ImageCacheSize
	if limit <= 0 {
		limit = 100 * 1024 * 1024
	}
	s.cache = diskCache{dir: dir, limit: limit}
	if cfg.RedisEnabled && cfg.RedisURL != "" {
		if rc, err := newRedisCache(context.Background(), cfg.RedisURL, log); err == nil {
			s.cache = rc
			log.Info("artwork cache: redis")
		} else {
			log.Warn("redis unavailable, artwork cache on disk", "err", err)
		}
	}
	return s, nil
}

// Serve writes the artwork for an entity (album/artist/playlist/song id),
// resized to fit the requested size.
func (s *Service) Serve(w http.ResponseWriter, r *http.Request, entityID string, size int) {
	if size <= 0 {
		size = 300
	}
	if size > 1024 {
		size = 1024
	}
	if size < 32 {
		size = 32
	}

	cacheKey := entityID + "-" + strconv.Itoa(size)
	if data, ok := s.cache.Get(r.Context(), cacheKey); ok {
		s.writeImage(w, data)
		return
	}

	art, err := s.resolve(r.Context(), entityID)
	if err != nil {
		s.log.Warn("artwork resolve failed", "id", entityID, "err", err)
		s.writeImage(w, s.placeholder)
		return
	}

	out := s.placeholder
	if art != nil {
		if resized := s.resize(art.Data, size); resized != nil {
			out = resized
		}
	}
	s.cache.Set(r.Context(), cacheKey, out)
	s.writeImage(w, out)
}

func (s *Service) writeImage(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// resolve finds artwork for an entity, following the fallback chain:
// song (custom photo) -> album -> artist (first album) -> playlist (first
// song album) -> song album cover -> category (Postgres, then MinIO).
func (s *Service) resolve(ctx context.Context, entityID string) (*store.Art, error) {
	// Custom photo uploaded for a song (or album/artist entity directly).
	if art, ok, err := s.store.GetArt(ctx, "song", entityID); err != nil || ok {
		return art, err
	}
	if art, ok, err := s.store.GetArt(ctx, "album", entityID); err != nil || ok {
		return art, err
	}
	if albumID, err := s.store.FirstAlbumByArtist(ctx, entityID); err == nil {
		if art, ok, err := s.store.GetArt(ctx, "album", albumID); err != nil || ok {
			return art, err
		}
	}
	if albumID, err := s.store.FirstPlaylistAlbum(ctx, entityID); err == nil {
		if art, ok, err := s.store.GetArt(ctx, "album", albumID); err != nil || ok {
			return art, err
		}
	}
	if albumID, err := s.store.FirstSongAlbum(ctx, entityID); err == nil && albumID != "" {
		if art, ok, err := s.store.GetArt(ctx, "album", albumID); err != nil || ok {
			return art, err
		}
	}
	return s.resolveCategory(ctx, entityID)
}

// resolveCategory looks up the category cover: Postgres first, then the
// MinIO copy (covers/<id>.jpg) as a fallback/recovery path.
func (s *Service) resolveCategory(ctx context.Context, entityID string) (*store.Art, error) {
	if art, ok, err := s.store.GetArt(ctx, "category", entityID); err != nil || ok {
		return art, err
	}
	if s.stg == nil {
		return nil, nil
	}
	info, err := s.stg.Stat(ctx, "covers/"+entityID+".jpg")
	if err != nil || info.Size <= 0 {
		return nil, nil
	}
	rc, err := s.stg.Open(ctx, "covers/"+entityID+".jpg", 0, -1)
	if err != nil {
		return nil, nil
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, 20<<20))
	if err != nil || len(data) == 0 {
		return nil, nil
	}
	return &store.Art{Data: data, Mime: "image/jpeg"}, nil
}

// UploadAlbumPhoto validates and stores a custom album photo, overriding the
// embedded cover. The image is resized to at most 1024px and re-encoded as
// JPEG; every cached size is invalidated so the new photo shows immediately.
func (s *Service) UploadAlbumPhoto(ctx context.Context, albumID string, data []byte) error {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return errors.New("imagem inválida ou corrompida")
	}
	out := s.encode(s.fit(img, 1024))
	if out == nil {
		return errors.New("falha ao processar imagem")
	}
	if err := s.store.UpsertArt(ctx, "album", albumID, out, "image/jpeg"); err != nil {
		return err
	}
	s.invalidate(albumID)
	return nil
}

// DeleteAlbumPhoto removes a custom photo, restoring the embedded cover.
func (s *Service) DeleteAlbumPhoto(ctx context.Context, albumID string) error {
	if err := s.store.DeleteArt(ctx, "album", albumID); err != nil {
		return err
	}
	s.invalidate(albumID)
	return nil
}

// UploadSongPhoto validates and stores a custom photo for a song, overriding
// the album/embedded fallback. Resized to at most 1024px, re-encoded JPEG.
func (s *Service) UploadSongPhoto(ctx context.Context, songID string, data []byte) error {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return errors.New("imagem inválida ou corrompida")
	}
	out := s.encode(s.fit(img, 1024))
	if out == nil {
		return errors.New("falha ao processar imagem")
	}
	if err := s.store.UpsertArt(ctx, "song", songID, out, "image/jpeg"); err != nil {
		return err
	}
	s.invalidate(songID)
	return nil
}

// DeleteSongPhoto removes a custom song photo, restoring the fallback art.
func (s *Service) DeleteSongPhoto(ctx context.Context, songID string) error {
	if err := s.store.DeleteArt(ctx, "song", songID); err != nil {
		return err
	}
	s.invalidate(songID)
	return nil
}

// UploadCategoryPhoto validates and stores a category cover. The processed
// JPEG is saved to both the artworks table (Postgres) and the MinIO bucket
// (covers/<id>.jpg), so either storage can recover the other.
func (s *Service) UploadCategoryPhoto(ctx context.Context, categoryID string, data []byte) error {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return errors.New("imagem inválida ou corrompida")
	}
	out := s.encode(s.fit(img, 1024))
	if out == nil {
		return errors.New("falha ao processar imagem")
	}
	if err := s.store.UpsertArt(ctx, "category", categoryID, out, "image/jpeg"); err != nil {
		return err
	}
	if s.stg != nil {
		if err := s.stg.Put(ctx, "covers/"+categoryID+".jpg", int64(len(out)), "image/jpeg", bytes.NewReader(out)); err != nil {
			s.log.Warn("category cover minio put failed", "id", categoryID, "err", err)
		}
	}
	s.invalidate(categoryID)
	return nil
}

// DeleteCategoryPhoto removes the category cover from both storages.
func (s *Service) DeleteCategoryPhoto(ctx context.Context, categoryID string) error {
	if err := s.store.DeleteArt(ctx, "category", categoryID); err != nil {
		return err
	}
	if s.stg != nil {
		if err := s.stg.Remove(ctx, "covers/"+categoryID+".jpg"); err != nil {
			s.log.Warn("category cover minio remove failed", "id", categoryID, "err", err)
		}
	}
	s.invalidate(categoryID)
	return nil
}

// invalidate clears every cached size (disk + redis) for an entity.
func (s *Service) invalidate(entityID string) {
	matches, _ := filepath.Glob(filepath.Join(s.dir, entityID+"-*.jpg"))
	for _, m := range matches {
		os.Remove(m)
	}
	if rc, ok := s.cache.(*redisCache); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		keys, err := rc.client.Keys(ctx, "pm:art:"+entityID+"-*").Result()
		if err == nil && len(keys) > 0 {
			rc.client.Del(ctx, keys...)
		}
	}
}

func (s *Service) resize(data []byte, size int) []byte {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return s.encode(s.fit(img, size))
}

// fit scales an image to fit within a square of `size`, keeping aspect ratio.
func (s *Service) fit(img image.Image, size int) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == 0 || h == 0 {
		return nil
	}
	if w <= size && h <= size {
		return img
	}
	ratio := float64(size) / float64(max(w, h))
	nw, nh := int(float64(w)*ratio), int(float64(h)*ratio)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, xdraw.Over, nil)
	return dst
}

func (s *Service) encode(img image.Image) []byte {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil
	}
	return buf.Bytes()
}

func (s *Service) makePlaceholder() []byte {
	img := image.NewRGBA(image.Rect(0, 0, placeholderSize, placeholderSize))
	top := color.RGBA{30, 30, 38, 255}
	bottom := color.RGBA{13, 13, 18, 255}
	for y := 0; y < placeholderSize; y++ {
		t := float64(y) / placeholderSize
		c := color.RGBA{
			R: uint8(float64(top.R) + (float64(bottom.R)-float64(top.R))*t),
			G: uint8(float64(top.G) + (float64(bottom.G)-float64(top.G))*t),
			B: uint8(float64(top.B) + (float64(bottom.B)-float64(top.B))*t),
			A: 255,
		}
		xdraw.Draw(img, image.Rect(0, y, placeholderSize, y+1), &image.Uniform{C: c}, image.Point{}, xdraw.Src)
	}
	return s.encode(img)
}

// ---------- disk cache ----------

type diskCache struct {
	dir   string
	limit int64
}

func (c diskCache) Get(ctx context.Context, key string) ([]byte, bool) {
	data, err := os.ReadFile(filepath.Join(c.dir, key+".jpg"))
	return data, err == nil
}

func (c diskCache) Set(ctx context.Context, key string, data []byte) {
	path := filepath.Join(c.dir, key+".jpg")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	os.Rename(tmp, path)
	enforceLimit(c.dir, c.limit)
}

func enforceLimit(dir string, limit int64) {
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
