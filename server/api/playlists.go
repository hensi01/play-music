package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (api *Router) listPlaylists(w http.ResponseWriter, r *http.Request) {
	pls, err := api.ds.Playlist(r.Context()).GetAll()
	if err != nil {
		api.handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, mapPlaylists(pls))
}

func (api *Router) getPlaylist(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	pls, err := api.playlists.GetWithTracks(r.Context(), pid)
	if err != nil {
		api.handleError(w, r, err)
		return
	}
	songs := make([]PlaylistSong, 0, len(pls.Tracks))
	for _, t := range pls.Tracks {
		songs = append(songs, PlaylistSong{EntryID: t.ID, Song: toSong(&t.MediaFile)})
	}
	respondJSON(w, http.StatusOK, PlaylistDetail{Playlist: toPlaylist(pls), Songs: songs})
}

func (api *Router) createPlaylist(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string   `json:"name"`
		SongIDs []string `json:"songIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	pid, err := api.playlists.Create(r.Context(), "", body.Name, body.SongIDs)
	if err != nil {
		api.handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{"id": pid})
}

func (api *Router) updatePlaylist(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	var body struct {
		Name          *string `json:"name"`
		Comment       *string `json:"comment"`
		Public        *bool   `json:"public"`
		AddSongIDs    []string `json:"addSongIds"`
		RemoveIndexes []int   `json:"removeIndexes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := api.playlists.Update(r.Context(), pid, body.Name, body.Comment, body.Public, body.AddSongIDs, body.RemoveIndexes); err != nil {
		api.handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"id": pid})
}

func (api *Router) deletePlaylist(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	if err := api.playlists.Delete(r.Context(), pid); err != nil {
		api.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (api *Router) addPlaylistTracks(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	var body struct {
		SongIDs []string `json:"songIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if _, err := api.playlists.AddTracks(r.Context(), pid, body.SongIDs); err != nil {
		api.handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"id": pid})
}

func (api *Router) removePlaylistTrack(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	entryID := chi.URLParam(r, "entryId")
	if err := api.playlists.RemoveTracks(r.Context(), pid, []string{entryID}); err != nil {
		api.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (api *Router) reorderPlaylistTracks(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	var body struct {
		From int `json:"from"`
		To   int `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := api.playlists.ReorderTrack(r.Context(), pid, body.From, body.To); err != nil {
		api.handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"id": pid})
}
