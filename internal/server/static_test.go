package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"play-music/internal/config"
)

// testStaticServer builds a Server with an empty config (no env-driven
// injection) — handleStatic reads s.cfg for ND_* frontend/store settings.
func testStaticServer() *Server {
	return &Server{cfg: &config.Config{}}
}

// B-NEW-1: unknown /api/* paths used to fall through to the SPA shell
// (200 + HTML), masking missing endpoints. They must answer a JSON 404.
func TestStaticUnknownAPI404JSON(t *testing.T) {
	s := testStaticServer()
	h := s.handleStatic()
	for _, p := range []string{"/api/banana/xyz", "/api/nope", "/api", "/api/store/nonexistent", "/api/me/nope"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404", p, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Fatalf("%s: Content-Type = %q, want JSON", p, ct)
		}
		if !strings.Contains(rec.Body.String(), "error") {
			t.Fatalf("%s: body is not a JSON error: %q", p, rec.Body.String())
		}
	}
}

// B-NEW-1: unknown /assets/* paths must return a plain 404, never HTML
// (HTML for a script URL triggers "Refused to execute script").
func TestStaticUnknownAsset404(t *testing.T) {
	s := testStaticServer()
	h := s.handleStatic()
	for _, p := range []string{"/assets/nope.js", "/assets/missing/app.css", "/assets"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404", p, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); strings.HasPrefix(ct, "text/html") {
			t.Fatalf("%s: must not serve HTML for assets, got %q", p, ct)
		}
	}
}

// The SPA fallback must keep working for unknown non-API routes (the UI is a
// hash router, so any path renders the app shell).
func TestStaticSpaFallbackPreserved(t *testing.T) {
	s := testStaticServer()
	h := s.handleStatic()
	for _, p := range []string{"/", "/home", "/library", "/login", "/some/hash/route"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", p, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("%s: Content-Type = %q, want HTML", p, ct)
		}
	}
}

// Known assets (the app loads ./style.css, ./app.js, ... from the root) must
// still be served normally.
func TestStaticKnownAssets(t *testing.T) {
	s := testStaticServer()
	h := s.handleStatic()
	for _, p := range []string{"/app.js", "/style.css", "/sw.js", "/pwa.js", "/manifest.webmanifest", "/icon-192.png"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", p, rec.Code)
		}
	}
}

// The store config (ND_STORE_CONFIG_JSON) must be injected into /loja.html at
// boot; an empty/invalid env var must yield "{}" (no prices/checkout links).
func TestStaticLojaStoreConfigInjected(t *testing.T) {
	raw := `{"categories":{"Cristão":{"price":"R$ 9,90","url":"https://checkout.exemplo.com/cristao"}},"packs":[]}`
	s := &Server{cfg: &config.Config{StoreConfigJSON: raw}}
	rec := httptest.NewRecorder()
	s.handleStatic().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/loja.html", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "__STORE_CONFIG__") {
		t.Fatalf("placeholder __STORE_CONFIG__ ainda presente no HTML")
	}
	if !strings.Contains(body, `https://checkout.exemplo.com/cristao`) {
		t.Fatalf("config da loja não injetada no HTML: %q", body)
	}
}

func TestStaticLojaStoreConfigEmpty(t *testing.T) {
	s := testStaticServer() // StoreConfigJSON vazio -> "{}"
	rec := httptest.NewRecorder()
	s.handleStatic().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/loja.html", nil))
	body := rec.Body.String()
	if strings.Contains(body, "__STORE_CONFIG__") {
		t.Fatalf("placeholder __STORE_CONFIG__ ainda presente no HTML")
	}
	if !strings.Contains(body, "const STORE_CONFIG = {}") {
		t.Fatalf("STORE_CONFIG deveria cair para {}: %q", body)
	}
}

func TestStaticLojaStoreConfigInvalidJSON(t *testing.T) {
	s := &Server{cfg: &config.Config{StoreConfigJSON: "not-json"}}
	rec := httptest.NewRecorder()
	s.handleStatic().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/loja.html", nil))
	body := rec.Body.String()
	if !strings.Contains(body, "const STORE_CONFIG = {}") {
		t.Fatalf("JSON inválido deveria cair para {}: %q", body)
	}
}

// ND_FRONTEND_BASEURL must be injected into index.html as a JSON string
// literal (empty -> ""), with quotes escaped safely (json.Marshal escapes
// " as \" and < as \u003c, so a value cannot break out of the <script> tag).
func TestStaticIndexBaseURLInjected(t *testing.T) {
	s := &Server{cfg: &config.Config{FrontendBaseURL: `https://api.exemplo.com" onload="alert(1)</script>`}}
	rec := httptest.NewRecorder()
	s.handleStatic().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()
	if strings.Contains(body, "__FRONTEND_BASEURL__") {
		t.Fatalf("placeholder __FRONTEND_BASEURL__ ainda presente no HTML")
	}
	if !strings.Contains(body, `baseURL: "https://api.exemplo.com\" onload=\"alert(1)\u003c/script\u003e"`) {
		t.Fatalf("baseURL não injetado/escapado corretamente: %q", body)
	}
}

func TestStaticIndexBaseURLEmpty(t *testing.T) {
	s := testStaticServer() // FrontendBaseURL vazio -> ""
	rec := httptest.NewRecorder()
	s.handleStatic().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(rec.Body.String(), "baseURL: \"\"") {
		t.Fatalf("baseURL vazio deveria ser \"\": %q", rec.Body.String())
	}
}
