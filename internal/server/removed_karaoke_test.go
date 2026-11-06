package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemovedKaraokeRoutes(t *testing.T) {
	s := testStaticServer()
	s.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, path := range []string{"/api/karaokes", "/api/karaokes/test", "/api/karaoke/stream/test", "/api/admin/karaokes"} {
		r := httptest.NewRecorder()
		s.Handler().ServeHTTP(r, httptest.NewRequest(http.MethodGet, path, nil))
		if r.Code != http.StatusNotFound {
			t.Errorf("%s: got %d, want 404", path, r.Code)
		}
	}
}
