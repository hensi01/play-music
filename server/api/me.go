package api

import (
	"net/http"

	"github.com/hensi01/play-music/model/request"
)

func (api *Router) getMe(w http.ResponseWriter, r *http.Request) {
	user, ok := request.UserFrom(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	respondJSON(w, http.StatusOK, UserProfile{
		ID:       user.ID,
		Name:     user.Name,
		Username: user.UserName,
		Email:    user.Email,
		IsAdmin:  user.IsAdmin,
	})
}
