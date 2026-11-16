package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientLoginRejectsMixedCredentials(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"phone":"11999999999","username":"admin","password":"secret"}`))
	s.handleLogin(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestAdminLoginRejectsClientCredentials(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/admin/login", strings.NewReader(`{"phone":"11999999999"}`))
	s.handleAdminLogin(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestAdminLoginRequiresBothCredentials(t *testing.T) {
	s := &Server{}
	for _, payload := range []string{`{}`, `{"username":"admin"}`, `{"password":"secret"}`} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/admin/login", strings.NewReader(payload))
		s.handleAdminLogin(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("payload %s: status = %d, want %d", payload, rec.Code, http.StatusBadRequest)
		}
	}
}
