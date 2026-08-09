package server

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"play-music/internal/version"
	"play-music/web"
)

// handleStatic serves the embedded web UI at / with SPA fallback to
// index.html. The asset URLs carry a version query (?v=...) so browsers and
// CDNs (Cloudflare) always fetch the current build instead of a stale cached
// copy, and every static response revalidates (Cache-Control: no-cache).
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
	index = bytes.ReplaceAll(index, []byte("__ASSET_VERSION__"), []byte(version.Version))

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
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(index)
			return
		}
		if _, err := fs.Stat(fsys, p); err != nil {
			// Unknown /api/* path: answer a JSON 404, never the SPA shell.
			// The SPA fallback used to mask missing API endpoints (clients
			// got 200 + HTML instead of a 404 JSON contract error).
			if p == "api" || strings.HasPrefix(p, "api/") {
				writeError(w, http.StatusNotFound, "Não encontrado")
				return
			}
			// Unknown /assets/* path: plain 404, never HTML. Serving HTML
			// for a script URL makes browsers refuse it with "Refused to
			// execute script" (MIME mismatch).
			if p == "assets" || strings.HasPrefix(p, "assets/") {
				http.NotFound(w, r)
				return
			}
			// SPA fallback: every unknown non-API route serves the app shell.
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(index)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
