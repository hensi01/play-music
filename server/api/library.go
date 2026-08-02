package api

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/hensi01/play-music/model"
	"github.com/hensi01/play-music/server/filter"
)

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func albumSort(sortParam string) (sortField, order string) {
	switch sortParam {
	case "name":
		return "name", "asc"
	case "artist":
		return "artist", "asc"
	case "year":
		return "max_year", "desc"
	case "played":
		return "playCount", "desc"
	case "random":
		return "random", ""
	case "recent":
		return "recently_added", "desc"
	default:
		return "name", "asc"
	}
}

func (api *Router) listAlbums(w http.ResponseWriter, r *http.Request) {
	sortField, order := albumSort(r.URL.Query().Get("sort"))
	if o := r.URL.Query().Get("order"); o == "asc" || o == "desc" {
		order = o
	}
	opts := model.QueryOptions{
		Sort:   sortField,
		Order:  order,
		Max:    queryInt(r, "limit", 100),
		Offset: queryInt(r, "offset", 0),
	}

	albums, err := api.ds.Album(r.Context()).GetAll(opts)
	if err != nil {
		api.handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, mapAlbums(albums))
}

func (api *Router) getAlbum(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	al, err := api.ds.Album(ctx).Get(id)
	if err != nil {
		api.handleError(w, r, err)
		return
	}

	opts := filter.SongsByAlbum(id)
	opts.Max = 1000
	songs, err := api.ds.MediaFile(ctx).GetAll(opts)
	if err != nil {
		api.handleError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, AlbumDetail{Album: toAlbum(al), Songs: mapSongs(songs)})
}

func (api *Router) listArtists(w http.ResponseWriter, r *http.Request) {
	opts := model.QueryOptions{
		Sort:   "name",
		Order:  "asc",
		Max:    queryInt(r, "limit", 500),
		Offset: queryInt(r, "offset", 0),
	}
	artists, err := api.ds.Artist(r.Context()).GetAll(opts)
	if err != nil {
		api.handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, mapArtists(artists))
}

func (api *Router) getArtist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	ar, err := api.ds.Artist(ctx).Get(id)
	if err != nil {
		api.handleError(w, r, err)
		return
	}

	aopts := filter.AlbumsByArtistID(id)
	aopts.Max = 200
	albums, err := api.ds.Album(ctx).GetAll(aopts)
	if err != nil {
		api.handleError(w, r, err)
		return
	}

	sopts := filter.SongsByArtistID(id)
	sopts.Max = 500
	songs, err := api.ds.MediaFile(ctx).GetAll(sopts)
	if err != nil {
		api.handleError(w, r, err)
		return
	}
	// Top songs by play count.
	sort.SliceStable(songs, func(i, j int) bool { return songs[i].PlayCount > songs[j].PlayCount })
	if len(songs) > 10 {
		songs = songs[:10]
	}

	respondJSON(w, http.StatusOK, ArtistDetail{
		Artist:   toArtist(ar),
		Albums:   mapAlbums(albums),
		TopSongs: mapSongs(songs),
	})
}

func (api *Router) getSong(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	mf, err := api.ds.MediaFile(r.Context()).Get(id)
	if err != nil {
		api.handleError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toSong(mf))
}
