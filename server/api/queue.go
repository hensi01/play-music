package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/hensi01/play-music/model"
	"github.com/hensi01/play-music/model/request"
)

type Queue struct {
	Current  int    `json:"current"`
	Position int64  `json:"position"`
	Songs    []Song `json:"songs"`
}

func (api *Router) getQueue(w http.ResponseWriter, r *http.Request) {
	user, ok := request.UserFrom(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	q, err := api.ds.PlayQueue(r.Context()).RetrieveWithMediaFiles(user.ID)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		api.handleError(w, r, err)
		return
	}
	if q == nil {
		respondJSON(w, http.StatusOK, Queue{Current: 0, Position: 0, Songs: []Song{}})
		return
	}
	respondJSON(w, http.StatusOK, Queue{Current: q.Current, Position: q.Position, Songs: mapSongs(q.Items)})
}

func (api *Router) saveQueue(w http.ResponseWriter, r *http.Request) {
	user, ok := request.UserFrom(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	var body struct {
		Current  int      `json:"current"`
		Position int64    `json:"position"`
		SongIDs  []string `json:"songIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	q := &model.PlayQueue{
		UserID:   user.ID,
		Current:  body.Current,
		Position: body.Position,
		ChangedBy: "PlayMusicUI",
	}
	for _, sid := range body.SongIDs {
		q.Items = append(q.Items, model.MediaFile{ID: sid})
	}
	if err := api.ds.PlayQueue(r.Context()).Store(q); err != nil {
		api.handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}
