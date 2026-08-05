package server

import (
	"encoding/json"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/hensi01/play-music/conf"
	"github.com/hensi01/play-music/conf/mime"
	"github.com/hensi01/play-music/consts"
	"github.com/hensi01/play-music/log"
	"github.com/hensi01/play-music/model"
	"github.com/hensi01/play-music/utils/str"
)

func Index(ds model.DataStore, fs fs.FS) http.HandlerFunc {
	return serveIndex(ds, fs)
}

// Injects the config in the `index.html` template
func serveIndex(ds model.DataStore, fs fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := ds.User(r.Context()).CountAll()
		firstTime := c == 0 && err == nil

		t, err := getIndexTemplate(r, fs)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		appConfig := map[string]any{
			"version":                   consts.Version,
			"firstTime":                 firstTime,
			"variousArtistsId":          consts.VariousArtistsID,
			"baseURL":                   str.SanitizeText(strings.TrimSuffix(conf.Server.BasePath, "/")),
			"loginBackgroundURL":        str.SanitizeText(conf.Server.UILoginBackgroundURL),
			"welcomeMessage":            str.SanitizeHTML(conf.Server.UIWelcomeMessage),
			"maxSidebarPlaylists":       conf.Server.MaxSidebarPlaylists,
			"enableTranscodingConfig":   conf.Server.EnableTranscodingConfig,
			"enableFavourites":          conf.Server.EnableFavourites,
			"enableStarRating":          conf.Server.EnableStarRating,
			"defaultTheme":              conf.Server.DefaultTheme,
			"defaultLanguage":           conf.Server.DefaultLanguage,
			"defaultUIVolume":           conf.Server.DefaultUIVolume,
			"uiSearchDebounceMs":        conf.Server.UISearchDebounceMs,
			"uiCoverArtSize":            conf.Server.UICoverArtSize,
			"enableCoverAnimation":      conf.Server.EnableCoverAnimation,
			"enableNowPlaying":          conf.Server.EnableNowPlaying,
			"playbackReportIntervalMs":  conf.Server.UIPlaybackReportInterval.Milliseconds(),
			"gaTrackingId":              conf.Server.GATrackingID,
			"losslessFormats":           strings.ToUpper(strings.Join(mime.LosslessFormats, ",")),
			"devActivityPanel":          conf.Server.DevActivityPanel,
			"enableUserEditing":         conf.Server.EnableUserEditing,
			"enableArtworkUpload":       conf.Server.EnableArtworkUpload,
			"devSidebarPlaylists":       conf.Server.DevSidebarPlaylists,
			"devShowArtistPage":         conf.Server.DevShowArtistPage,
			"devUIShowConfig":           conf.Server.DevUIShowConfig,
			"devNewEventStream":         conf.Server.DevNewEventStream,
			"enableReplayGain":          conf.Server.EnableReplayGain,
			"defaultDownsamplingFormat": conf.Server.DefaultDownsamplingFormat,
			"separator":                 string(os.PathSeparator),
			"enableInspect":             conf.Server.Inspect.Enabled,
			"extAuthLogoutURL":          conf.Server.ExtAuth.LogoutURL,
		}
		if strings.HasPrefix(conf.Server.UILoginBackgroundURL, "/") {
			appConfig["loginBackgroundURL"] = path.Join(conf.Server.BasePath, conf.Server.UILoginBackgroundURL)
		}
		auth := handleLoginFromHeaders(ds, r)
		if auth != nil {
			appConfig["auth"] = auth
		}
		appConfigJson, err := json.Marshal(appConfig)
		if err != nil {
			log.Error(r, "Error converting config to JSON", "config", appConfig, err)
		} else {
			log.Trace(r, "Injecting config in index.html", "config", string(appConfigJson))
		}

		log.Debug("UI configuration", "appConfig", appConfig)
		version := consts.Version
		if version != "dev" {
			version = "v" + version
		}
		data := map[string]any{
			"AppConfig": template.JS(string(appConfigJson)),
			"Version":   version,
		}

		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		err = t.Execute(w, data)
		if err != nil {
			log.Error(r, "Could not execute `index.html` template", err)
		}
	}
}

func getIndexTemplate(r *http.Request, fs fs.FS) (*template.Template, error) {
	t := template.New("initial state")
	indexHtml, err := fs.Open("index.html")
	if err != nil {
		log.Error(r, "Could not find `index.html` template", err)
		return nil, err
	}
	indexStr, err := io.ReadAll(indexHtml)
	if err != nil {
		log.Error(r, "Could not read from `index.html`", err)
		return nil, err
	}
	t, err = t.Parse(string(indexStr))
	if err != nil {
		log.Error(r, "Error parsing `index.html`", err)
		return nil, err
	}
	return t, nil
}
