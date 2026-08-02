// Package api implements the Play Music REST API consumed by the web UI.
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hensi01/play-music/core/artwork"
	"github.com/hensi01/play-music/core/lyrics"
	"github.com/hensi01/play-music/core/playlists"
	"github.com/hensi01/play-music/core/scrobbler"
	"github.com/hensi01/play-music/core/stream"
	"github.com/hensi01/play-music/log"
	"github.com/hensi01/play-music/model"
	"github.com/hensi01/play-music/model/request"
	"github.com/hensi01/play-music/server"
)

// Router exposes the Play Music API under the /api mount point.
type Router struct {
	http.Handler
	ds               model.DataStore
	artwork          artwork.Artwork
	streamer         stream.MediaStreamer
	transcodeDecider stream.TranscodeDecider
	playlists        playlists.Playlists
	playTracker      scrobbler.PlayTracker
	lyrics           lyrics.Lyrics
}

func New(ds model.DataStore, artwork artwork.Artwork, streamer stream.MediaStreamer,
	transcodeDecider stream.TranscodeDecider, playlists playlists.Playlists,
	playTracker scrobbler.PlayTracker, lyrics lyrics.Lyrics) *Router {
	r := &Router{
		ds:               ds,
		artwork:          artwork,
		streamer:         streamer,
		transcodeDecider: transcodeDecider,
		playlists:        playlists,
		playTracker:      playTracker,
		lyrics:           lyrics,
	}
	r.Handler = r.routes()
	return r
}

func (api *Router) routes() http.Handler {
	r := chi.NewRouter()

	r.Group(func(r chi.Router) {
		r.Use(server.Authenticator(api.ds))
		r.Use(server.JWTRefresher)
		r.Use(server.UpdateLastAccessMiddleware(api.ds))
		r.Use(webClientMiddleware)

		r.Get("/me", api.getMe)
		r.Get("/settings", api.getSettings)
		r.Get("/home", api.getHome)
		r.Get("/search", api.search)

		r.Get("/albums", api.listAlbums)
		r.Get("/albums/{id}", api.getAlbum)
		r.Get("/artists", api.listArtists)
		r.Get("/artists/{id}", api.getArtist)
		r.Get("/songs/{id}", api.getSong)

		r.Get("/playlists", api.listPlaylists)
		r.Get("/playlists/{id}", api.getPlaylist)
		r.Post("/playlists", api.createPlaylist)
		r.Put("/playlists/{id}", api.updatePlaylist)
		r.Delete("/playlists/{id}", api.deletePlaylist)
		r.Post("/playlists/{id}/tracks", api.addPlaylistTracks)
		r.Delete("/playlists/{id}/tracks/{entryId}", api.removePlaylistTrack)
		r.Put("/playlists/{id}/tracks", api.reorderPlaylistTracks)

		r.Get("/me/liked", api.getLikedSongs)
		r.Put("/me/liked/{id}", api.likeSong)
		r.Delete("/me/liked/{id}", api.unlikeSong)
		r.Get("/me/history", api.getHistory)
		r.Post("/me/history/{id}", api.registerPlay)

		r.Get("/queue", api.getQueue)
		r.Put("/queue", api.saveQueue)

		r.Get("/stream/{id}", api.stream)
		r.Get("/artwork/{id}", api.getArtwork)
		r.Get("/lyrics/{id}", api.getLyrics)
	})

	return r
}

// webClientMiddleware attaches a synthetic Player to the context so play tracking
// and transcoding decisions have a client identity without the Subsonic client
// negotiation layer.
func webClientMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := model.Player{
			Client:          "PlayMusicUI",
			Name:            "PlayMusicUI [" + r.UserAgent() + "]",
			UserAgent:       r.UserAgent(),
			IP:              r.RemoteAddr,
			ScrobbleEnabled: false,
		}
		ctx := request.WithPlayer(r.Context(), p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func (api *Router) handleError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, model.ErrNotFound):
		respondError(w, http.StatusNotFound, "not found")
	case errors.Is(err, model.ErrNotAuthorized):
		respondError(w, http.StatusForbidden, "not authorized")
	default:
		log.Error(r, "API error", err)
		respondError(w, http.StatusInternalServerError, "internal error")
	}
}
