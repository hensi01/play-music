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
// only stream songs from granted categories. The Bunny CDN is the ONLY
// delivery path: native formats are 302-redirected to a signed CDN URL and
// non-native formats (or an explicit format=mp3) are transcoded with ffmpeg.
// There is no local proxy fallback — if the CDN is not configured the
// endpoint answers 500.
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

	// Native format: CDN-only — always redirect to the signed Bunny CDN URL.
	// No probe, no ?nocdn=1, no local proxy. A missing/disabled CDN is a
	// server error, not a silent fallback.
	url, err := s.stream.StreamURL(ctx, song)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}
