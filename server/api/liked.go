package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hensi01/play-music/server/filter"
)

func (api *Router) getLikedSongs(w http.ResponseWriter, r *http.Request) {
	opts := filter.ByStarred()
	opts.Max = 500
	songs, err := api.ds.MediaFile(r.Context()).GetAll(opts)
	if err != nil {
		api.handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, mapSongs(songs))
}

func (api *Router) likeSong(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := api.ds.MediaFile(r.Context()).SetStar(true, id); err != nil {
		api.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (api *Router) unlikeSong(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := api.ds.MediaFile(r.Context()).SetStar(false, id); err != nil {
		api.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
