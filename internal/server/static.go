package server

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"play-music/internal/version"
	"play-music/web"
)

type versionedAsset struct {
	body        []byte
	contentType string
}

// handleStatic serves the embedded web UI at / with SPA fallback to index.html.
// Asset URLs carry a version query (?v=...) so browsers and CDNs fetch the
// current build; every static response revalidates.
func (s *Server) handleStatic() http.Handler {
	fsys, err := fs.Sub(web.Assets, "assets")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServerFS(fsys)
	index, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		panic(err)
	}
	baseURL, _ := json.Marshal(s.cfg.FrontendBaseURL)
	index = bytes.ReplaceAll(index, []byte("__ASSET_VERSION__"), []byte(version.Version))
	index = bytes.ReplaceAll(index, []byte("__FRONTEND_BASEURL__"), baseURL)

	substituted := func(name string) []byte {
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			panic(err)
		}
		return bytes.ReplaceAll(body, []byte("__ASSET_VERSION__"), []byte(version.Version))
	}

	// The service worker and the manifest also name assets by version, so they
	// get the same substitution instead of a copy of the number. The content
	// type travels with the name: these are its only two users.
	versioned := map[string]versionedAsset{
		"sw.js":                {body: substituted("sw.js"), contentType: "text/javascript; charset=utf-8"},
		"manifest.webmanifest": {body: substituted("manifest.webmanifest"), contentType: "application/manifest+json"},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "." || p == "" {
			p = "index.html"
		}
		w.Header().Set("Cache-Control", "no-cache")
		if p == "index.html" {
			writeHTML(w, index)
			return
		}
		if asset, ok := versioned[p]; ok {
			w.Header().Set("Content-Type", asset.contentType)
			w.Write(asset.body)
			return
		}

		if _, err := fs.Stat(fsys, p); err != nil {
			// A JSON 404 for /api/*, never the SPA shell, so a missing endpoint
			// cannot look like a successful HTML page.
			if p == "api" || strings.HasPrefix(p, "api/") {
				writeError(w, http.StatusNotFound, "Não encontrado")
				return
			}
			// A missing document is a 404; only extensionless paths are
			// client-side routes of the SPA.
			if p == "assets" || strings.HasPrefix(p, "assets/") || strings.HasSuffix(p, ".html") {
				http.NotFound(w, r)
				return
			}
			writeHTML(w, index)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

func writeHTML(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(body)
}
