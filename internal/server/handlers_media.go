package server

import (
	"net/http"

	"play-music/internal/artwork"
	"play-music/internal/stream"
)

func (s *Server) handleArtwork(w http.ResponseWriter, r *http.Request) {
	s.artwork.Serve(w, r, r.PathValue("id"), parseIntQuery(r, "size", artwork.DefaultSize))
}

// handleStream redirects to a time-limited storage URL, or transcodes with
// ffmpeg when the browser cannot play the format. mp3 is the only target.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	song, err := s.store.GetSong(ctx, r.PathValue("id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}

	transcode := !stream.IsNative(song.Format)
	if format := r.URL.Query().Get("format"); format != "" && format != song.Format {
		if format != stream.TranscodeFormat {
			writeError(w, http.StatusBadRequest, "Formato de transcodificação não suportado")
			return
		}
		transcode = true
	}
	if transcode {
		if err := s.stream.Transcode(ctx, w, r, song); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	url, err := s.stream.StreamURL(ctx, song)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	http.Redirect(w, r, url, http.StatusFound)
}
