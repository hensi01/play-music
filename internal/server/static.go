package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"play-music/web"
)

// handleStatic serves the embedded web UI at / with SPA fallback to
// index.html.
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

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "." || p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(fsys, p); err != nil {
			// SPA fallback: every unknown route serves the app shell.
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.Write(index)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
