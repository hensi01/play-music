package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// getLyrics returns the synced/plain lyrics for a media file, when available.
func (api *Router) getLyrics(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	mf, err := api.ds.MediaFile(r.Context()).Get(id)
	if err != nil {
		api.handleError(w, r, err)
		return
	}

	lyr, err := api.lyrics.GetLyrics(r.Context(), mf)
	if err != nil || len(lyr) == 0 {
		respondJSON(w, http.StatusOK, LyricsResponse{Lines: []LyricLine{}})
		return
	}

	main, ok := lyr.Main()
	if !ok {
		respondJSON(w, http.StatusOK, LyricsResponse{Lines: []LyricLine{}})
		return
	}

	resp := LyricsResponse{Synced: main.Synced, Lines: []LyricLine{}}
	for _, l := range main.Line {
		resp.Lines = append(resp.Lines, LyricLine{Start: l.Start, End: l.End, Text: l.Value})
	}
	respondJSON(w, http.StatusOK, resp)
}
