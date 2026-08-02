package subsonic

import (
	"net/http"
	"strings"

	"github.com/hensi01/play-music/conf"
	"github.com/hensi01/play-music/model"
	"github.com/hensi01/play-music/model/request"
	"github.com/hensi01/play-music/server/subsonic/responses"
	"github.com/hensi01/play-music/utils/req"
	"github.com/hensi01/play-music/utils/slice"
)

// buildUserResponse creates a User response object from a User model
func buildUserResponse(user model.User) responses.User {
	userResponse := responses.User{
		Username:          user.UserName,
		AdminRole:         user.IsAdmin,
		Email:             user.Email,
		StreamRole:        true,
		ScrobblingEnabled: true,
		DownloadRole:      conf.Server.EnableDownloads,
		ShareRole:         conf.Server.EnableSharing,
		CoverArtRole:      conf.Server.EnableArtworkUpload || user.IsAdmin,
		Folder:            slice.Map(user.Libraries, func(lib model.Library) int32 { return int32(lib.ID) }),
	}

	if conf.Server.Jukebox.Enabled {
		userResponse.JukeboxRole = !conf.Server.Jukebox.AdminOnly || user.IsAdmin
	}

	return userResponse
}

func (api *Router) GetUser(r *http.Request) (*responses.Subsonic, error) {
	loggedUser, ok := request.UserFrom(r.Context())
	if !ok {
		return nil, newError(responses.ErrorGeneric, "Internal error")
	}
	username, err := req.Params(r).String("username")
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(username, loggedUser.UserName) {
		return nil, newError(responses.ErrorAuthorizationFail)
	}
	response := newResponse()
	response.User = new(buildUserResponse(loggedUser))
	return response, nil
}

func (api *Router) GetUsers(r *http.Request) (*responses.Subsonic, error) {
	loggedUser, ok := request.UserFrom(r.Context())
	if !ok {
		return nil, newError(responses.ErrorGeneric, "Internal error")
	}

	user := buildUserResponse(loggedUser)
	response := newResponse()
	response.Users = &responses.Users{User: []responses.User{user}}
	return response, nil
}
