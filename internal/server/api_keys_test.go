package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The public API docs must be served, and every admin route documented in them
// must be reachable only with an API key (or a dashboard session), never open.
func TestPublicSwaggerFilesAndAdminSecurity(t *testing.T) {
	h := testStaticServer().handleStatic()
	for _, path := range []string{"/docs/", "/docs/swagger-ui-bundle.js", "/docs/swagger-ui.css", "/docs/docs.js"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/docs/openapi.json", nil))
	var spec struct {
		Paths map[string]map[string]struct {
			Security []map[string][]string `json:"security"`
		} `json:"paths"`
	}
	if w.Code != 200 {
		t.Fatalf("/docs/openapi.json: %d", w.Code)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
		t.Fatalf("invalid OpenAPI document: %v", err)
	}
	upload := spec.Paths["/api/admin/songs"]["post"]
	found := false
	for _, sec := range upload.Security {
		if _, ok := sec["ApiKey"]; ok {
			found = true
		}
	}
	if !found {
		t.Fatal("upload sem autenticação por chave na documentação")
	}
	for path, methods := range spec.Paths {
		if !strings.HasPrefix(path, "/api/admin/") && path != "/api/scan" {
			continue
		}
		for method, op := range methods {
			hasKey := false
			for _, sec := range op.Security {
				if _, ok := sec["ApiKey"]; ok {
					hasKey = true
				}
			}
			if !hasKey {
				t.Fatalf("admin sem chave em %s %s", method, path)
			}
		}
	}
}

func TestAdminRejectsMalformedKey(t *testing.T) {
	s := &Server{}
	called := false
	h := s.requireAdmin(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	for _, key := range []string{"bad", "pmk_" + strings.Repeat("!", 43), "pmk_" + strings.Repeat("a", 42)} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/admin/songs", nil)
		r.Header.Set("X-API-Key", key)
		h.ServeHTTP(w, r)
		if w.Code != 401 || called {
			t.Fatalf("chave inválida recebida: status %d", w.Code)
		}
	}
}

// Every registered route must be documented, and the spec must not advertise
// paths the server does not serve.
func TestOpenAPIAndRoutesAgree(t *testing.T) {
	w := httptest.NewRecorder()
	testStaticServer().handleStatic().ServeHTTP(w, httptest.NewRequest("GET", "/docs/openapi.json", nil))
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	registered := map[string]bool{}
	for _, rt := range (&Server{}).routes() {
		method, path, ok := strings.Cut(rt.Pattern, " ")
		if !ok {
			t.Fatalf("route without method: %q", rt.Pattern)
		}
		key := strings.ToLower(method) + " " + path
		registered[key] = true
		if len(spec.Paths[path][strings.ToLower(method)]) == 0 {
			t.Errorf("rota sem documentação: %s", key)
		}
	}
	for path, methods := range spec.Paths {
		for method := range methods {
			if !registered[method+" "+path] {
				t.Errorf("documentação sem rota: %s %s", method, path)
			}
		}
	}
}
