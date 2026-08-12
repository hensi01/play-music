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

// handleStatic serves the embedded web UI at / with SPA fallback to
// index.html. The asset URLs carry a version query (?v=...) so browsers and
// CDNs (Cloudflare) always fetch the current build instead of a stale cached
// copy, and every static response revalidates (Cache-Control: no-cache).
//
// Environment-driven config is injected at boot, never hardcoded in the
// frontend: ND_FRONTEND_BASEURL -> index.html (window.__APP_CONFIG__.baseURL)
// and ND_STORE_CONFIG_JSON -> loja.html (STORE_CONFIG).
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
	index = bytes.ReplaceAll(index, []byte("__FRONTEND_BASEURL__"), quoteJSON(s.cfg.FrontendBaseURL))

	loja, err := fs.ReadFile(fsys, "loja.html")
	if err != nil {
		panic(err)
	}
	loja = bytes.ReplaceAll(loja, []byte("__STORE_CONFIG__"), storeConfigJSON(s.cfg.StoreConfigJSON))

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
		if p == "loja.html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(loja)
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

// quoteJSON serializes a plain string as a JSON string literal (HTML-safe).
// Used to inject ND_FRONTEND_BASEURL into index.html; an empty env var
// becomes "" (same-origin API), escaping quotes/newlines safely.
func quoteJSON(v string) []byte {
	b, _ := json.Marshal(v)
	return b
}

// storeConfigJSON returns the ND_STORE_CONFIG_JSON value verbatim when it is
// valid JSON, or "{}" when empty/invalid (no prices/checkout links shown).
func storeConfigJSON(raw string) []byte {
	raw = strings.TrimSpace(raw)
	if raw != "" && json.Valid([]byte(raw)) {
		return []byte(raw)
	}
	return []byte("{}")
}
