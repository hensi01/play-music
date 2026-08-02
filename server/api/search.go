package api

import (
	"net/http"
	"strings"

	"github.com/hensi01/play-music/log"
	"github.com/hensi01/play-music/model"
)

func (api *Router) search(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		respondJSON(w, http.StatusOK, SearchResults{})
		return
	}

	results := SearchResults{
		Songs:     []Song{},
		Albums:    []Album{},
		Artists:   []Artist{},
		Playlists: []Playlist{},
	}

	if songs, err := api.ds.MediaFile(ctx).Search(q, model.QueryOptions{Max: 30}); err == nil {
		results.Songs = mapSongs(songs)
	} else {
		log.Warn(ctx, "Error searching songs", "q", q, err)
	}

	if albums, err := api.ds.Album(ctx).Search(q, model.QueryOptions{Max: 20}); err == nil {
		results.Albums = mapAlbums(albums)
	} else {
		log.Warn(ctx, "Error searching albums", "q", q, err)
	}

	if artists, err := api.ds.Artist(ctx).Search(q, model.QueryOptions{Max: 20}); err == nil {
		results.Artists = mapArtists(artists)
	} else {
		log.Warn(ctx, "Error searching artists", "q", q, err)
	}

	// Playlists have no full-text index; match by name.
	if pls, err := api.ds.Playlist(ctx).GetAll(); err == nil {
		lower := strings.ToLower(q)
		for i := range pls {
			if strings.Contains(strings.ToLower(pls[i].Name), lower) {
				results.Playlists = append(results.Playlists, toPlaylist(&pls[i]))
				if len(results.Playlists) >= 10 {
					break
				}
			}
		}
	}

	respondJSON(w, http.StatusOK, results)
}
