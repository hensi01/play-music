package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"play-music/internal/auth"
	"play-music/internal/config"
	"play-music/internal/db"
	"play-music/internal/store"
)

func TestAPIKeyLifecyclePostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not configured for disposable database")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = db.Migrate(ctx, pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	id := store.NewID()
	_, err = pool.Exec(ctx, `INSERT INTO users(id,name,password_hash,is_admin) VALUES($1,'integration test','unused',true)`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", id) }()
	_, err = pool.Exec(ctx, `INSERT INTO push_subscriptions(endpoint,user_id,relay_id,management_token,subscription_data,active) VALUES($1,$2,$3,'token','{}',true)`, "https://example.invalid/"+id, id, "test-"+id)
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(pool)
	authService, err := auth.New(ctx, &config.Config{}, st)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{store: st, auth: authService}
	r := httptest.NewRequest("POST", "/api/admin/api-keys", strings.NewReader(`{"name":"test"}`))
	r = r.WithContext(context.WithValue(r.Context(), ctxUserID, id))
	w := httptest.NewRecorder()
	s.handleCreateAPIKey(w, r)
	if w.Code != 201 {
		t.Fatalf("create: %d", w.Code)
	}
	var key struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	if json.Unmarshal(w.Body.Bytes(), &key) != nil || key.ID == "" || key.Key == "" {
		t.Fatal("no one-time key")
	}
	w = httptest.NewRecorder()
	s.handleListAPIKeys(w, r)
	if strings.Contains(w.Body.String(), key.Key) {
		t.Fatal("secret returned in listing")
	}
	called := false
	h := s.requireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if userIDOf(r.Context()) != id {
			t.Error("wrong owner")
		}
		w.WriteHeader(204)
	}))
	upload := func() int {
		called = false
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/admin/songs", nil)
		r.Header.Set("X-API-Key", key.Key)
		h.ServeHTTP(w, r)
		return w.Code
	}
	if code := upload(); code != 204 || !called {
		t.Fatalf("valid key: %d", code)
	}
	// A key that is valid for admin routes must not authenticate user routes.
	userRoute := s.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("key reached user handler") }))
	other := httptest.NewRequest("GET", "/api/me", nil)
	other.Header.Set("X-API-Key", key.Key)
	otherResult := httptest.NewRecorder()
	userRoute.ServeHTTP(otherResult, other)
	if otherResult.Code != 401 {
		t.Fatalf("key on user route: %d", otherResult.Code)
	}
	_, err = pool.Exec(ctx, "UPDATE users SET is_admin=false WHERE id=$1", id)
	if err != nil {
		t.Fatal(err)
	}
	if code := upload(); code != 401 || called {
		t.Fatalf("demoted owner: %d", code)
	}
	_, err = pool.Exec(ctx, "UPDATE users SET is_admin=true WHERE id=$1", id)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, "UPDATE push_subscriptions SET active=false WHERE user_id=$1", id)
	if err != nil {
		t.Fatal(err)
	}
	if code := upload(); code != 401 || called {
		t.Fatalf("inactive notifications: %d", code)
	}
	_, err = pool.Exec(ctx, "UPDATE push_subscriptions SET active=true WHERE user_id=$1", id)
	if err != nil {
		t.Fatal(err)
	}
	revoke := httptest.NewRequest("DELETE", "/api/admin/api-keys/"+key.ID, nil)
	revoke.SetPathValue("id", key.ID)
	w = httptest.NewRecorder()
	s.handleRevokeAPIKey(w, revoke)
	if w.Code != 204 {
		t.Fatalf("revoke: %d", w.Code)
	}
	if code := upload(); code != 401 || called {
		t.Fatalf("revoked key: %d", code)
	}
}
