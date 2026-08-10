package server

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"play-music/internal/metadata"
	"play-music/internal/scanner"
	"play-music/internal/store"
)

// ---------- karaoke (client view) ----------

// handleKaraokes lists every karaoke accessible to the user (Karaoke page).
func (s *Server) handleKaraokes(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.AllKaraokes(r.Context(), s.filterUser(r))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// handleKaraoke returns one karaoke. Clients may only open karaokes from
// categories granted to them (403 otherwise).
func (s *Server) handleKaraoke(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := parseID(r)
	if userID := s.filterUser(r); userID != "" {
		ok, err := s.store.CanAccessKaraoke(ctx, userID, id)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		if !ok {
			writeError(w, http.StatusForbidden, "Sem permissão")
			return
		}
	}
	k, err := s.store.GetKaraoke(ctx, id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, k)
}

// handleKaraokeStream serves GET /api/karaoke/stream/{id}. Non-admin users can
// only stream karaokes from granted categories. Videos are served locally with
// HTTP Range support (same cache pipeline as audio).
func (s *Server) handleKaraokeStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := parseID(r)
	if userID := s.filterUser(r); userID != "" {
		ok, err := s.store.CanAccessKaraoke(ctx, userID, id)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		if !ok {
			writeError(w, http.StatusForbidden, "Sem permissão")
			return
		}
	}
	k, err := s.store.GetKaraoke(ctx, id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if err := s.stream.ServeVideo(ctx, w, r, k); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

// handleKaraokeRegisterPlay records a karaoke play (play counter).
func (s *Server) handleKaraokeRegisterPlay(w http.ResponseWriter, r *http.Request) {
	if err := s.store.RegisterKaraokePlay(r.Context(), userIDOf(r.Context()), s.filterUser(r), parseID(r)); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

// ---------- admin: karaoke management ----------

// handleAdminKaraokes lists every karaoke plus the category ids each one
// belongs to (admin karaoke management screen).
func (s *Server) handleAdminKaraokes(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.AllKaraokes(r.Context(), "")
	if err != nil {
		handleStoreError(w, err)
		return
	}
	cats, err := s.store.KaraokeCategoryIDs(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	categoryList, err := s.store.GetCategories(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"karaokes":      list,
		"categoryIds":   cats,
		"categoryList":  categoryList,
	})
}

// videoMimeByExt maps accepted video extensions to their upload Content-Type.
var videoMimeByExt = map[string]string{
	".mp4":  "video/mp4",
	".m4v":  "video/mp4",
	".webm": "video/webm",
	".mkv":  "video/x-matroska",
}

// handleAdminUploadKaraoke accepts a multipart form with:
//   - "video" (file, required) — MP4 (or webm/mkv) file to add to the library
//   - "title"/"artist" (optional) — metadata overrides
//   - "categoryId" (optional) — category the karaoke is assigned to
//   - "photo" (optional) — karaoke photo (jpg/png/webp), overrides the thumbnail
//
// The file is stored under uploads/ in the bucket and indexed immediately
// (duration probe + auto thumbnail via ffmpeg).
func (s *Server) handleAdminUploadKaraoke(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "Arquivo muito grande (máx 512MB) ou inválido")
		return
	}
	file, header, err := r.FormFile("video")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Campo 'video' obrigatório (arquivo de vídeo)")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	contentType := videoMimeByExt[ext]
	if contentType == "" {
		writeError(w, http.StatusBadRequest, "Formato de vídeo não suportado: "+ext+" (use .mp4, .webm ou .mkv)")
		return
	}

	// Buffer to a temp file so we know the size before Put.
	tmp, err := os.CreateTemp("", "pm-karaoke-upload-*"+ext)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Falha ao processar arquivo")
		return
	}
	defer os.Remove(tmp.Name())
	size, err := io.Copy(tmp, file)
	if err != nil || size == 0 {
		tmp.Close()
		writeError(w, http.StatusBadRequest, "Falha ao ler arquivo")
		return
	}
	if err := tmp.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, "Falha ao processar arquivo")
		return
	}

	// Validate the payload is actually decodable video BEFORE it hits the
	// bucket (same guard as the song upload: junk renamed to .mp4 is
	// rejected without touching storage).
	ffprobe := metadata.ProbePath(metadata.ConfiguredFFmpegPath())
	duration, _, _ := metadata.Probe(tmp.Name(), ffprobe)
	if duration <= 0 {
		writeError(w, http.StatusBadRequest, "Arquivo de vídeo inválido ou corrompido")
		return
	}

	key := "uploads/" + store.NewID() + ext
	in, err := os.Open(tmp.Name())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Falha ao processar arquivo")
		return
	}
	defer in.Close()
	if err := s.storage.Put(r.Context(), key, size, contentType, in); err != nil {
		s.log.Error("karaoke upload put", "key", key, "err", err)
		writeError(w, http.StatusInternalServerError, "Falha ao enviar para o armazenamento")
		return
	}

	k, err := s.scanner.IndexKaraoke(r.Context(), key, header.Filename)
	if err != nil {
		s.log.Warn("karaoke index (retry)", "key", key, "err", err)
		s.waitForObject(r.Context(), key)
		k, err = s.scanner.IndexKaraoke(r.Context(), key, header.Filename)
	}
	if err != nil {
		s.log.Error("karaoke index", "key", key, "err", err)
		// The object was already Put into the bucket: remove it so a failed
		// upload does not leave an orphaned object behind.
		if rerr := s.storage.Remove(r.Context(), key); rerr != nil {
			s.log.Warn("karaoke upload cleanup", "key", key, "err", rerr)
		}
		if errors.Is(err, scanner.ErrInvalidVideo) {
			writeError(w, http.StatusBadRequest, "Arquivo de vídeo inválido ou corrompido")
			return
		}
		writeError(w, http.StatusInternalServerError, "Falha ao indexar o karaokê")
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	artist := strings.TrimSpace(r.FormValue("artist"))
	if title != "" || artist != "" {
		if err := s.store.UpdateKaraokeMeta(r.Context(), k.ID, title, artist); err != nil {
			s.log.Warn("karaoke meta update", "err", err)
		}
	}

	if categoryID := strings.TrimSpace(r.FormValue("categoryId")); categoryID != "" {
		if err := s.store.AddKaraokeToCategory(r.Context(), categoryID, k.ID); err != nil {
			s.log.Warn("karaoke category assign", "err", err)
		}
	}

	if photo, _, err := r.FormFile("photo"); err == nil {
		data, err := io.ReadAll(io.LimitReader(photo, maxPhotoBytes))
		photo.Close()
		if err == nil && len(data) > 0 {
			if err := s.artwork.UploadKaraokePhoto(r.Context(), k.ID, data); err != nil {
				s.log.Warn("karaoke photo", "err", err)
			}
		}
	}

	writeJSON(w, http.StatusCreated, k)
}

// handleAdminUploadKaraokePhoto uploads a custom photo for a karaoke.
func (s *Server) handleAdminUploadKaraokePhoto(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ok, err := s.store.KaraokeExists(r.Context(), id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "Não encontrado")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxPhotoBytes)
	if err := r.ParseMultipartForm(maxPhotoBytes); err != nil {
		writeError(w, http.StatusBadRequest, "Imagem muito grande (máx 15MB)")
		return
	}
	file, _, err := r.FormFile("photo")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Campo 'photo' obrigatório")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPhotoBytes))
	if err != nil || len(data) == 0 {
		writeError(w, http.StatusBadRequest, "Falha ao ler imagem")
		return
	}
	if err := s.artwork.UploadKaraokePhoto(r.Context(), id, data); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeNoContent(w)
}

// handleAdminDeleteKaraokePhoto removes the custom photo, restoring the
// auto-generated thumbnail.
func (s *Server) handleAdminDeleteKaraokePhoto(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ok, err := s.store.KaraokeExists(r.Context(), id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "Não encontrado")
		return
	}
	if err := s.artwork.DeleteKaraokePhoto(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeNoContent(w)
}
