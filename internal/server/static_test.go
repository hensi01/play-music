package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"play-music/internal/config"
	"play-music/internal/stream"
)

// testStaticServer builds a Server whose config carries no env-driven
// FrontendBaseURL.
func testStaticServer() *Server {
	return &Server{cfg: &config.Config{}}
}

// A missing endpoint must answer a JSON 404, never the SPA shell: a 200+HTML
// response makes a missing API look like a successful page.
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

// A missing /assets/* path is a plain 404, never HTML: HTML for a script URL
// makes browsers refuse it with a MIME mismatch.
func TestStaticUnknownAsset404(t *testing.T) {
	s := testStaticServer()
	h := s.handleStatic()
	for _, p := range []string{"/assets/nope.js", "/assets/missing/app.css", "/assets", "/loja.html"} {
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
	for _, p := range []string{"/", "/home", "/library", "/login", "/admin/login", "/some/hash/route"} {
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

// sw.js and manifest.webmanifest are the two files that name assets by build
// version, so they are substituted and must carry a non-generic content type.
func TestStaticVersionedAssetsSubstituted(t *testing.T) {
	s := testStaticServer()
	h := s.handleStatic()
	for _, tc := range []struct{ path, contentType string }{
		{"/sw.js", "text/javascript"},
		{"/manifest.webmanifest", "application/manifest+json"},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", tc.path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, tc.contentType) {
			t.Fatalf("%s: Content-Type = %q, want %q", tc.path, ct, tc.contentType)
		}
		if body := rec.Body.String(); strings.Contains(body, "__ASSET_VERSION__") {
			t.Fatalf("%s: version placeholder left in body", tc.path)
		}
	}
}

// Known assets (the app loads ./style.css, ./app.js, ... from the root) must
// still be served normally.
func TestStaticKnownAssets(t *testing.T) {
	s := testStaticServer()
	h := s.handleStatic()
	for _, p := range []string{"/app.js", "/style.css", "/sw.js", "/pwa.js", "/platform.js", "/manifest.webmanifest", "/icon-192.png"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", p, rec.Code)
		}
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

func TestSettingsListsUploadableAudioExtensions(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Server{}).handleSettings(rec, httptest.NewRequest("GET", "/api/settings", nil))
	var body struct {
		AudioExtensions []string `json:"audioExtensions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	// The admin folder uploader filters on this list, so the endpoint must
	// carry exactly what the upload and stream paths accept.
	if !reflect.DeepEqual(body.AudioExtensions, stream.AudioExtensions()) {
		t.Fatalf("audioExtensions = %v, want %v", body.AudioExtensions, stream.AudioExtensions())
	}
}

func TestSettingsDoesNotExposeStorageCredentials(t *testing.T) {
	s := &Server{cfg: &config.Config{S3: config.S3Config{
		Endpoint: "minio.example.com", AccessKey: "private-user", SecretKey: "private-secret", Bucket: "music",
	}}}
	rec := httptest.NewRecorder()
	s.handleSettings(rec, httptest.NewRequest("GET", "/api/settings", nil))
	body := rec.Body.String()
	if strings.Contains(body, "private-user") || strings.Contains(body, "private-secret") || strings.Contains(body, "minio.example.com") {
		t.Fatalf("storage connection exposed in settings: %s", body)
	}
}
