package api

import (
	"context"
	"net/http"

	"github.com/hensi01/play-music/log"
	"github.com/hensi01/play-music/model"
	"github.com/hensi01/play-music/server/filter"
)

func (api *Router) getHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	home := Home{Genres: []Genre{}}

	if section, ok := api.albumSection(ctx, "recent", "Tocados recentemente", filter.AlbumsByRecent(), 10); ok {
		home.Sections = append(home.Sections, section)
	}
	if section, ok := api.albumSection(ctx, "new", "Novos lançamentos", filter.AlbumsByNewest(), 10); ok {
		home.Sections = append(home.Sections, section)
	}
	if section, ok := api.albumSection(ctx, "frequent", "Mais tocados", filter.AlbumsByFrequent(), 10); ok {
		home.Sections = append(home.Sections, section)
	}
	if section, ok := api.albumSection(ctx, "random", "Sugestões para você", filter.AlbumsByRandom(), 10); ok {
		home.Sections = append(home.Sections, section)
	}

	genres, err := api.ds.Genre(ctx).GetAll()
	if err != nil {
		log.Warn(ctx, "Error loading genres for home", err)
	} else {
		for _, g := range genres {
			home.Genres = append(home.Genres, Genre{Name: g.Name, SongCount: g.SongCount, AlbumCount: g.AlbumCount})
		}
	}

	respondJSON(w, http.StatusOK, home)
}

func (api *Router) albumSection(ctx context.Context, id, title string, opts model.QueryOptions, max int) (Section, bool) {
	opts.Max = max
	albums, err := api.ds.Album(ctx).GetAll(opts)
	if err != nil || len(albums) == 0 {
		return Section{}, false
	}
	return Section{ID: id, Title: title, Albums: mapAlbums(albums)}, true
}
