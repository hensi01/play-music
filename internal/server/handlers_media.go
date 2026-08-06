package server

import (
	"net/http"
)

// handleArtwork serves GET /api/artwork/{id}?size=N. The id can be an album,
// artist, playlist or song id; the JWT may travel as ?jwt=. Access is checked
// for non-admin users (artwork of hidden content returns 403).
func (s *Server) handleArtwork(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := parseID(r)
	if userID := s.filterUser(r); userID != "" {
		ok, err := s.store.CanAccessEntity(ctx, userID, id)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		if !ok {
			writeError(w, http.StatusForbidden, "Sem permissão")
			return
		}
	}
	size := parseIntQuery(r, "size", 300)
	s.artwork.Serve(w, r, id, size)
}

// handleStream serves GET /api/stream/{id}?format=mp3. Non-admin users can
// only stream songs from granted categories. Native formats are redirected to
// a signed URL (Bunny CDN or MinIO presigned); other formats (or an explicit
// format=mp3) are transcoded with ffmpeg.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	songID := parseID(r)
	if userID := s.filterUser(r); userID != "" {
		ok, err := s.store.CanAccessSong(ctx, userID, songID)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		if !ok {
			writeError(w, http.StatusForbidden, "Sem permissão")
			return
		}
	}
	song, err := s.store.GetSong(ctx, songID)
	if err != nil {
		handleStoreError(w, err)
		return
	}

	format := r.URL.Query().Get("format")
	if format != "" && format != song.Format {
		// Transcode requested (e.g. format=mp3 for non-native files).
		if format != "mp3" {
			writeError(w, http.StatusBadRequest, "Formato de transcodificação não suportado")
			return
		}
		if err := s.stream.Transcode(ctx, w, r, song); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	if !nativeFormats[song.Format] {
		if err := s.stream.Transcode(ctx, w, r, song); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	url, err := s.stream.StreamURL(ctx, song)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Falha ao gerar URL de reprodução")
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}
