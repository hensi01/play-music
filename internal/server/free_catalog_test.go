package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"play-music/internal/auth"
	"play-music/internal/config"
	"play-music/internal/db"
	"play-music/internal/store"
)

func TestFreeCatalogAndRecommendationsIntegration(t *testing.T) {
	url := os.Getenv("PM_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("isolated database not configured")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var dbname string
	if err = pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbname); err != nil || !strings.HasPrefix(dbname, "codex_free_") {
		t.Fatal("test requires an isolated codex_free_ database")
	}
	st := store.New(pool)
	cfg := &config.Config{AdminUsername: "isolated-admin", AdminPassword: "isolated-password"}
	authsvc, err := auth.New(ctx, cfg, st)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{store: st, auth: authsvc, cfg: cfg, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	handler := s.Handler()
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set(auth.HeaderName, "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	register := func(phone string) string {
		t.Helper()
		rec := call("POST", "/api/register", "", `{"phone":"`+phone+`"}`)
		if rec.Code != 200 {
			t.Fatalf("register: %d %s", rec.Code, rec.Body.String())
		}
		var v struct {
			Token string `json:"token"`
		}
		if json.Unmarshal(rec.Body.Bytes(), &v) != nil || v.Token == "" {
			t.Fatal("missing token")
		}
		return v.Token
	}
	tokenA := register("11987654321")
	tokenB := register("11987654322")
	if rec := call("POST", "/api/register", "", `{"phone":"11987654321"}`); rec.Code != 409 {
		t.Fatalf("duplicate registration: %d", rec.Code)
	}
	if rec := call("GET", "/api/recommendations", "", ""); rec.Code != 401 {
		t.Fatalf("recommendations require login: %d", rec.Code)
	}
	if rec := call("GET", "/loja.html", "", ""); rec.Code != 404 {
		t.Fatalf("old shop: %d", rec.Code)
	}
	if rec := call("POST", "/api/store/purchase", tokenA, `{}`); rec.Code != 404 && rec.Code != 405 {
		t.Fatalf("old purchase: %d", rec.Code)
	}
	artistID, err := st.GetOrCreateArtist(ctx, "Isolated Artist")
	if err != nil {
		t.Fatal(err)
	}
	song1, song2 := store.NewID(), store.NewID()
	_, err = pool.Exec(ctx, `INSERT INTO songs(id,path,title,artist,artist_id,genre,duration,format) VALUES ($1,$2,'First','Isolated Artist',$3,'Rock',90,'mp3'),($4,$5,'Second','Other',NULL,'Jazz',120,'mp3')`, song1, "test/"+song1, artistID, song2, "test/"+song2)
	if err != nil {
		t.Fatal(err)
	}
	if rec := call("GET", "/api/artists/"+artistID, tokenB, ""); rec.Code != 200 {
		t.Fatalf("artist without album: %d", rec.Code)
	}
	fallbackSong := store.NewID()
	if _, err = pool.Exec(ctx, `INSERT INTO songs(id,path,title,artist,artist_id,duration,format) VALUES($1,$2,'Album artist fallback','',$3,90,'mp3')`, fallbackSong, "test/"+fallbackSong, artistID); err != nil {
		t.Fatal(err)
	}
	fallbackRec := call("GET", "/api/songs/"+fallbackSong, tokenB, "")
	var fallbackPayload struct {
		Artist string `json:"artist"`
	}
	if json.Unmarshal(fallbackRec.Body.Bytes(), &fallbackPayload) != nil || fallbackPayload.Artist != "Isolated Artist" {
		t.Fatalf("missing album artist fallback: %s", fallbackRec.Body)
	}
	cat, err := st.CreateCategory(ctx, "Future Category "+song1)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.AddSongToCategory(ctx, cat.ID, song1); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{tokenA, tokenB} {
		for _, path := range []string{"/api/songs/" + song1, "/api/songs/" + song2, "/api/categories/" + cat.ID} {
			if rec := call("GET", path, token, ""); rec.Code != 200 {
				t.Fatalf("free access %s: %d %s", path, rec.Code, rec.Body.String())
			}
		}
	}
	if rec := call("PUT", "/api/me/liked/"+song1, tokenA, ""); rec.Code != 204 {
		t.Fatalf("like: %d", rec.Code)
	}
	body := `{"songId":"` + song1 + `","playId":"play-unique-1234567890"}`
	for i := 0; i < 2; i++ {
		if rec := call("POST", "/api/me/listens", tokenA, body); rec.Code != 204 {
			t.Fatalf("qualified listen: %d %s", rec.Code, rec.Body.String())
		}
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM qualified_listens WHERE song_id=$1", song1).Scan(&count); err != nil || count != 1 {
		t.Fatalf("idempotency count=%d error=%v", count, err)
	}
	ra, rb := call("GET", "/api/recommendations", tokenA, ""), call("GET", "/api/recommendations", tokenB, "")
	var a, b struct {
		Personalized bool `json:"personalized"`
		Songs        []struct {
			ID    string `json:"id"`
			Liked bool   `json:"liked"`
		} `json:"songs"`
	}
	if json.NewDecoder(bytes.NewReader(ra.Body.Bytes())).Decode(&a) != nil || json.NewDecoder(bytes.NewReader(rb.Body.Bytes())).Decode(&b) != nil || !a.Personalized || b.Personalized {
		t.Fatalf("profiles not isolated: %s / %s", ra.Body, rb.Body)
	}
	for _, song := range b.Songs {
		if song.Liked {
			t.Fatal("another user's like leaked")
		}
	}
	rec := call("GET", "/api/songs/"+song1, tokenB, "")
	var payload struct {
		Liked bool `json:"liked"`
	}
	if json.Unmarshal(rec.Body.Bytes(), &payload) != nil || payload.Liked {
		t.Fatalf("song like leaked: %s", rec.Body)
	}
	oldAlbum, err := st.GetOrCreateAlbum(ctx, "Demo Album", "Isolated Artist", artistID, 2020)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE songs SET album='Demo Album',album_id=$2 WHERE id=$1", song1, oldAlbum); err != nil {
		t.Fatal(err)
	}
	if err = st.UpsertArt(ctx, "album", oldAlbum, []byte("original-cover"), "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	if err = st.UpdateSongMeta(ctx, song1, "", "New Band"); err != nil {
		t.Fatal(err)
	}
	updated, err := st.GetSong(ctx, song1)
	if err != nil {
		t.Fatal(err)
	}
	newArtist, err := st.GetArtist(ctx, updated.ArtistID)
	if err != nil || newArtist.Name != "New Band" || updated.AlbumID == oldAlbum {
		t.Fatalf("artist metadata inconsistent: %+v %v", updated, err)
	}
	cover, found, err := st.GetArt(ctx, "album", updated.AlbumID)
	if err != nil || !found || string(cover.Data) != "original-cover" {
		t.Fatal("album artwork not preserved")
	}
	if rec := call("GET", "/api/artists/"+updated.ArtistID, tokenB, ""); rec.Code != 200 {
		t.Fatalf("free artist access: %d", rec.Code)
	}
	otherArtist, err := st.GetOrCreateArtist(ctx, "Other Band")
	if err != nil {
		t.Fatal(err)
	}
	otherAlbum, err := st.GetOrCreateAlbum(ctx, "Demo Album", "Other Band", otherArtist, 2020)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.UpsertArt(ctx, "album", otherAlbum, []byte("existing-cover"), "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	if err = st.UpdateSongMeta(ctx, song1, "", "Other Band"); err != nil {
		t.Fatal(err)
	}
	cover, found, err = st.GetArt(ctx, "album", otherAlbum)
	if err != nil || !found || string(cover.Data) != "existing-cover" {
		t.Fatal("existing album cover was overwritten")
	}

}
