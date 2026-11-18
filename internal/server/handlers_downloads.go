package server

import (
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"play-music/internal/model"
)

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	song, err := s.store.GetSong(ctx, r.PathValue("id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	exists, err := s.storage.ObjectExists(ctx, song.Path)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "Armazenamento indisponível; tente novamente")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "Arquivo da música indisponível")
		return
	}
	link, err := s.storage.DownloadURL(ctx, song.Path, attachmentDisposition(song), 15*time.Minute)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Não foi possível preparar o download")
		return
	}
	if err = s.store.RecordDownload(ctx, userIDOf(ctx), song.ID, song.Title, song.Artist); err != nil {
		handleStoreError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, map[string]string{"url": link})
}

func attachmentDisposition(song *model.Song) string {
	name := strings.TrimSpace(song.Title)
	if name == "" {
		name = "musica"
	}
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, name)
	return mime.FormatMediaType("attachment", map[string]string{"filename": name + path.Ext(song.Path)})
}

func (s *Server) handleDownloads(w http.ResponseWriter, r *http.Request) {
	downloads, err := s.store.Downloads(r.Context(), userIDOf(r.Context()))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, downloads)
}
