package api

import (
	"net/http"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/go-chi/chi/v5"
	"github.com/hensi01/play-music/core/scrobbler"
	"github.com/hensi01/play-music/model"
)

func (api *Router) getHistory(w http.ResponseWriter, r *http.Request) {
	opts := model.QueryOptions{
		Sort:    "play_date",
		Order:   "desc",
		Filters: squirrel.Gt{"play_date": time.Time{}},
		Max:     50,
	}
	songs, err := api.ds.MediaFile(r.Context()).GetAll(opts)
	if err != nil {
		api.handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, mapSongs(songs))
}

func (api *Router) registerPlay(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := api.playTracker.Submit(r.Context(), []scrobbler.Submission{{TrackID: id, Timestamp: time.Now()}})
	if err != nil {
		api.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
