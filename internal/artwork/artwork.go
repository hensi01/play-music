package artwork

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
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
	"play-music/internal/fscache"
	"play-music/internal/storage"
	"play-music/internal/store"
)

const (
	placeholderSize = 640
	maxStoredSize   = 1024
)

// artTTL bounds how long a cached cover lives. writeImage's Cache-Control
// derives from it, so the browser and the server cache expire together.
const artTTL = 24 * time.Hour

type Service struct {
	store       *store.Store
	stg         *storage.Storage
	log         *slog.Logger
	placeholder []byte
	cache       cache
}

// DefaultSize is the artwork side length used when a request names none. The
// handler's default and the service's clamp must agree, so both read this.
const DefaultSize = 300

type cache interface {
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, data []byte)
	Invalidate(entityID string)
}

func New(cfg *config.Config, st *store.Store, stg *storage.Storage, log *slog.Logger) (*Service, error) {
	dir := "var/cache/art"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Service{store: st, stg: stg, log: log}
	s.placeholder = s.makePlaceholder()
	limit := cfg.ImageCacheSize
	if limit <= 0 {
		limit = 100 * 1024 * 1024
	}
	s.cache = diskCache{dir: dir, limit: limit}
	if cfg.RedisEnabled && cfg.RedisURL != "" {
		if rc, err := newRedisCache(context.Background(), cfg.RedisURL); err == nil {
			s.cache = rc
			log.Info("artwork cache: redis")
		} else {
			log.Warn("redis unavailable, artwork cache on disk", "err", err)
		}
	}
	return s, nil
}

// cacheKey names one resized cover: the entity id plus its size. Both cache
// implementations store under it and both invalidators match by
// cacheKeyPrefix, so the shape is defined here and nowhere else.
func cacheKey(entityID string, size int) string {
	return entityID + "-" + strconv.Itoa(size)
}

// cacheKeyPrefix is the start of every cached size of one entity.
func cacheKeyPrefix(entityID string) string {
	return entityID + "-"
}

func (s *Service) Serve(w http.ResponseWriter, r *http.Request, entityID string, size int) {
	if size <= 0 {
		size = DefaultSize
	}
	if size > maxStoredSize {
		size = maxStoredSize
	}
	if size < 32 {
		size = 32
	}

	key := cacheKey(entityID, size)
	if data, ok := s.cache.Get(r.Context(), key); ok {
		s.writeImage(w, data)
		return
	}

	out := s.placeholder
	art, err := s.resolve(r.Context(), entityID)
	if err != nil {
		s.log.Warn("artwork resolve failed", "id", entityID, "err", err)
		s.writeImage(w, s.placeholder)
		return
	}
	if art != nil {
		if resized := s.resize(art.Data, size); resized != nil {
			out = resized
		}
	}
	s.cache.Set(r.Context(), key, out)
	s.writeImage(w, out)
}

func (s *Service) writeImage(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(int(artTTL.Seconds())))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *Service) resolve(ctx context.Context, entityID string) (*store.Art, error) {
	for _, entityType := range []string{store.EntitySong, store.EntityAlbum} {
		if art, ok, err := s.store.GetArt(ctx, entityType, entityID); err != nil || ok {
			return art, err
		}
	}
	albumLookups := []func() (string, error){
		func() (string, error) { return s.store.FirstAlbumByArtist(ctx, entityID) },
		func() (string, error) { return s.store.FirstPlaylistAlbum(ctx, entityID) },
		func() (string, error) { return s.store.FirstSongAlbum(ctx, entityID) },
	}
	for _, albumOf := range albumLookups {
		albumID, err := albumOf()
		if err != nil || albumID == "" {
			continue
		}
		if art, ok, err := s.store.GetArt(ctx, store.EntityAlbum, albumID); err != nil || ok {
			return art, err
		}
	}
	return s.resolveCover(ctx, store.EntityCategory, entityID)
}

// mirrorKey is the bucket copy of an entity's cover, or "" when that entity type
// is not mirrored. Categories are the only mirrored type: the scanner writes
// category covers into the bucket, and the category row is what lets either
// store recover the other, so the two must not diverge.
func mirrorKey(entityType, entityID string) string {
	if entityType != store.EntityCategory {
		return ""
	}
	return "covers/" + entityID + ".jpg"
}

// resolveCover reads a cover from whichever store can supply it. A mirrored type
// is looked up in Postgres first and then in the bucket.
func (s *Service) resolveCover(ctx context.Context, entityType, entityID string) (*store.Art, error) {
	if art, ok, err := s.store.GetArt(ctx, entityType, entityID); err != nil || ok {
		return art, err
	}
	key := mirrorKey(entityType, entityID)
	if key == "" || s.stg == nil {
		return nil, nil
	}
	info, err := s.stg.Stat(ctx, key)
	if err != nil || info.Size <= 0 {
		return nil, nil
	}
	rc, err := s.stg.Open(ctx, key, 0, -1)
	if err != nil {
		return nil, nil
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, 20<<20))
	if err != nil || len(data) == 0 {
		return nil, nil
	}
	return &store.Art{Data: data}, nil
}

// Put stores already-encoded cover bytes for an entity and drops every cached
// size of it. This is the only writer of artwork rows other than Delete, so a
// cover discovered after the placeholder was cached cannot stay stale.
func (s *Service) Put(ctx context.Context, entityType, entityID string, data []byte) error {
	if err := s.store.UpsertArt(ctx, entityType, entityID, data); err != nil {
		return err
	}
	s.cache.Invalidate(entityID)
	return nil
}

// Upload stores a validated, resized image as the entity's custom photo.
// Mirrored types are also written to the bucket, so either store can recover the
// other.
func (s *Service) Upload(ctx context.Context, entityType, entityID string, data []byte) error {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return errors.New("imagem inválida ou corrompida")
	}
	out := s.encode(s.fit(img, maxStoredSize))
	if out == nil {
		return errors.New("falha ao processar imagem")
	}
	if err := s.Put(ctx, entityType, entityID, out); err != nil {
		return err
	}
	if key := mirrorKey(entityType, entityID); key != "" && s.stg != nil {
		if err := s.stg.Put(ctx, key, int64(len(out)), "image/jpeg", bytes.NewReader(out)); err != nil {
			s.log.Warn("cover mirror put failed", "type", entityType, "id", entityID, "err", err)
		}
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, entityType, entityID string) error {
	if err := s.store.DeleteArt(ctx, entityType, entityID); err != nil {
		return err
	}
	if key := mirrorKey(entityType, entityID); key != "" && s.stg != nil {
		if err := s.stg.Remove(ctx, key); err != nil {
			s.log.Warn("cover mirror remove failed", "type", entityType, "id", entityID, "err", err)
		}
	}
	s.cache.Invalidate(entityID)
	return nil
}

func (s *Service) resize(data []byte, size int) []byte {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return s.encode(s.fit(img, size))
}

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
	fscache.EnforceLimit(c.dir, c.limit)
}

func (c diskCache) Invalidate(entityID string) {
	matches, _ := filepath.Glob(filepath.Join(c.dir, filepath.Base(cacheKeyPrefix(entityID))+"*.jpg"))
	for _, m := range matches {
		os.Remove(m)
	}
}
