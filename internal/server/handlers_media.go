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

// handleStream serves GET /api/stream/{id}?format=mp3&nocdn=1. Non-admin
// users can only stream songs from granted categories. Native formats are
// served directly through this server with HTTP Range support (so seeking
// works even though the Bunny CDN ignores Range requests for uncached
// content); other formats (or an explicit format=mp3) are transcoded with
// ffmpeg.
//
// Native formats prefer the Bunny CDN when its pull zone answers Range
// requests (probed with a fresh signed URL): the client is 302-redirected to
// the signed CDN URL. ?nocdn=1 skips the CDN entirely and proxies the bytes
// locally (client-side fallback when the CDN URL fails to play).
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

	// Native format: prefer the CDN when its pull zone handles Range
	// requests on cache misses; otherwise proxy the bytes locally so
	// seeking always works. ?nocdn=1 (client fallback after a CDN
	// failure) forces the local proxy.
	if r.URL.Query().Get("nocdn") != "1" && s.stream.CDNRangeOK(ctx, song.Path) {
		url, err := s.stream.StreamURL(ctx, song)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Falha ao gerar URL de reprodução")
			return
		}
		http.Redirect(w, r, url, http.StatusFound)
		return
	}
	if err := s.stream.ServeNative(ctx, w, r, song); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
