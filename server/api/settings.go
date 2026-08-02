package api

import (
	"net/http"

	"github.com/hensi01/play-music/conf"
	"github.com/hensi01/play-music/consts"
)

func (api *Router) getSettings(w http.ResponseWriter, r *http.Request) {
	libraryName := "Música"
	libraries, err := api.ds.Library(r.Context()).GetAll()
	if err == nil && len(libraries) > 0 {
		libraryName = libraries[0].Name
	}
	respondJSON(w, http.StatusOK, Settings{
		AppName:     "Play Music",
		Version:     consts.Version,
		LibraryName: libraryName,
		MusicFolder: conf.Server.MusicFolder,
	})
}
