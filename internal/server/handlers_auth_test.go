package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The two login routes accept disjoint credentials: the phone route is for
// clients, the admin route for username/password. Each rejects a payload that
// mixes them.
func TestLoginRoutesRejectMixedCredentials(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
		body    string
	}{
		{"client route with phone and password", (&Server{}).handleLogin, `{"phone":"11999999999","password":"secret"}`},
		{"client route with phone and username", (&Server{}).handleLogin, `{"phone":"11999999999","username":"admin"}`},
		{"admin route with phone", (&Server{}).handleAdminLogin, `{"phone":"11999999999"}`},
		{"admin route without username", (&Server{}).handleAdminLogin, `{"password":"secret"}`},
		{"admin route without password", (&Server{}).handleAdminLogin, `{"username":"admin"}`},
		{"admin route empty", (&Server{}).handleAdminLogin, `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.handler(rec, httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(tc.body)))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}
