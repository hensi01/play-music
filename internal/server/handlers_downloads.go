package server

import (
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := parseID(r)
	if user := s.filterUser(r); user != "" {
		ok, err := s.store.CanAccessSong(ctx, user, id)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		if !ok {
			writeError(w, http.StatusForbidden, "Sem permissão")
			return
		}
	}
	song, err := s.store.GetSong(ctx, id)
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
	ext := path.Ext(song.Path)
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
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": name + ext})
	link, err := s.storage.DownloadURL(ctx, song.Path, disposition, 15*time.Minute)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Não foi possível preparar o download")
		return
	}
	_, err = s.store.Pool().Exec(ctx, "INSERT INTO download_history(user_id,song_id,title,artist) VALUES($1,$2,$3,$4)", userIDOf(ctx), id, song.Title, song.Artist)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, map[string]string{"url": link})
}

func (s *Server) handleDownloads(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.Pool().Query(r.Context(), `SELECT COALESCE(d.song_id, ''), d.title, d.artist, d.downloaded_at FROM download_history d WHERE d.user_id=$1 ORDER BY d.downloaded_at DESC, d.id DESC LIMIT 500`, userIDOf(r.Context()))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	defer rows.Close()
	type entry struct {
		ID           string    `json:"id"`
		Title        string    `json:"title"`
		Artist       string    `json:"artist"`
		DownloadedAt time.Time `json:"downloadedAt"`
	}
	result := []entry{}
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.ID, &e.Title, &e.Artist, &e.DownloadedAt); err != nil {
			handleStoreError(w, err)
			return
		}
		result = append(result, e)
	}
	if err := rows.Err(); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
