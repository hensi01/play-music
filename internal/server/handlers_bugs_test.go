package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"play-music/internal/artwork"
	"play-music/internal/config"
	"play-music/internal/db"
	"play-music/internal/model"
	"play-music/internal/scanner"
	"play-music/internal/storage"
	"play-music/internal/store"
)

// newTestServerStore builds a Server against a throwaway database. Every test
// here writes rows, so it must not run against the database the app uses.
func newTestServerStore(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	pool, err := db.Connect(context.Background(), testDisposableDB(t))
	if err != nil {
		t.Skipf("postgres unreachable: %v", err)
		return nil, nil
	}
	t.Cleanup(pool.Close)
	st := store.New(pool)
	return &Server{store: st, cfg: &config.Config{}, log: slog.Default()}, st
}

// newFullTestServer builds a Server with the real wiring (store + storage +
// scanner) so upload happy paths can be exercised end-to-end. The database is
// the throwaway one the other integration tests use; only the S3 settings come
// from the app configuration, because uploads need a bucket to write to.
func newFullTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	url := testDisposableDB(t)
	for k, v := range dotEnv(t) {
		if os.Getenv(k) == "" {
			t.Setenv(k, v)
		}
	}
	cfg := config.Load()
	if cfg.S3.Endpoint == "" || cfg.S3.AccessKey == "" || cfg.S3.SecretKey == "" {
		t.Skip("S3/MinIO config missing — integration test skipped")
	}
	pool, err := db.Connect(context.Background(), url)
	if err != nil {
		t.Skipf("postgres unreachable: %v", err)
		return nil, nil
	}
	t.Cleanup(pool.Close)
	st := store.New(pool)
	strg, err := storage.New(cfg.S3)
	if err != nil {
		t.Skipf("storage init: %v", err)
		return nil, nil
	}
	artSvc, err := artwork.New(cfg, st, strg, slog.Default())
	if err != nil {
		t.Skipf("artwork init: %v", err)
		return nil, nil
	}
	sc := scanner.New(cfg, strg, st, artSvc, slog.Default())
	return &Server{cfg: cfg, store: st, storage: strg, scanner: sc, log: slog.Default()}, st
}

// uniquePhone returns a valid 11-digit phone number unlikely to collide with
// rows left by earlier runs (the create endpoint rejects duplicates).
func uniquePhone(t *testing.T) string {
	t.Helper()
	digits := strconv.FormatInt(time.Now().UnixNano(), 10)
	return "11" + digits[len(digits)-9:]
}

// omitempty does not omit a zero time.Time, so the create response must carry
// a real createdAt instead of "0001-01-01T00:00:00Z".
func TestAdminCreateUserReturnsCreatedAt(t *testing.T) {
	s, st := newTestServerStore(t)

	phone := uniquePhone(t)
	body := `{"name":"Test Worker","phone":"` + phone + `"}`
	rec := httptest.NewRecorder()
	s.handleAdminCreateUser(rec,
		httptest.NewRequest(http.MethodPost, "/api/admin/users", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var u model.User
	if err := json.Unmarshal(rec.Body.Bytes(), &u); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	t.Cleanup(func() { _ = st.DeleteUser(context.Background(), u.ID) })

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	created, _ := raw["createdAt"].(string)
	if created == "" || created == "0001-01-01T00:00:00Z" {
		t.Fatalf("createdAt not echoed with a real timestamp: %q", created)
	}
}

// Liking something that does not exist must answer 404: a silent 204 desyncs
// the client's like state.
func TestLikeUnknownSongReturns404(t *testing.T) {
	s, _ := newTestServerStore(t)
	ctx := context.WithValue(context.Background(), ctxUserID, store.NewID())
	unknown := store.NewID()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/me/liked/"+unknown, nil).WithContext(ctx)
	req.SetPathValue("id", unknown)
	s.handleLikeSong(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("like unknown: status = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/me/liked/"+unknown, nil).WithContext(ctx)
	req.SetPathValue("id", unknown)
	s.handleUnlikeSong(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unlike unknown: status = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}
}

// A real song id must still like/unlike with 204.
func TestLikeExistingSongReturns204(t *testing.T) {
	s, st := newTestServerStore(t)
	songs, err := st.AllSongs(context.Background())
	if err != nil || len(songs) == 0 {
		t.Skipf("no songs in the catalog — skipping: %v", err)
	}
	songID := songs[0].ID
	ctx := context.WithValue(context.Background(), ctxUserID, store.NewID())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/me/liked/"+songID, nil).WithContext(ctx)
	req.SetPathValue("id", songID)
	s.handleLikeSong(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("like existing: status = %d, want 204 (body %s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/me/liked/"+songID, nil).WithContext(ctx)
	req.SetPathValue("id", songID)
	s.handleUnlikeSong(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("unlike existing: status = %d, want 204 (body %s)", rec.Code, rec.Body.String())
	}
}

// multipartSong builds a multipart/form-data body with a single "song" file.
func multipartSong(t *testing.T, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("song", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

// A file renamed to .mp3 whose content is not decodable audio must be rejected
// before it reaches the bucket, so nothing is inserted into the catalog.
func TestAdminUploadRejectsInvalidAudio(t *testing.T) {
	s, st := newTestServerStore(t)
	before, err := st.AllSongs(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	body, contentType := multipartSong(t, "fake.mp3", bytes.Repeat([]byte("x"), 1024))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/songs", body)
	req.Header.Set("Content-Type", contentType)
	s.handleAdminUploadSong(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if msg, _ := resp["error"].(string); !strings.Contains(strings.ToLower(msg), "áudio inválido") {
		t.Fatalf("error message does not mention invalid audio: %q", msg)
	}
	after, err := st.AllSongs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("catalog changed by the rejected upload: before=%d after=%d", len(before), len(after))
	}
}

// A valid untagged WAV must pass the pre-Put validation and be indexed with a
// real duration (>0). Cleaning up uses the same prune the scanner performs.
func TestAdminUploadValidWavHasDurationAndCreatedAt(t *testing.T) {
	s, st := newFullTestServer(t)
	before, err := st.AllSongs(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var songPath string
	defer func() {
		if songPath == "" {
			return
		}
		paths, err := st.LoadSongPaths(context.Background())
		if err != nil {
			t.Logf("cleanup of uploaded WAV song: %v", err)
			return
		}
		delete(paths, songPath)
		keep := make([]string, 0, len(paths))
		for path := range paths {
			keep = append(keep, path)
		}
		if _, err := st.DeleteMissing(context.Background(), keep); err != nil {
			t.Logf("cleanup of uploaded WAV song: %v", err)
		}
	}()

	body, contentType := multipartSong(t, "pw-test-valid.wav", testWAV(t, 1))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/songs", body)
	req.Header.Set("Content-Type", contentType)
	s.handleAdminUploadSong(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	var song model.Song
	if err := json.Unmarshal(rec.Body.Bytes(), &song); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	songPath = song.Path
	if song.Duration <= 0 {
		t.Fatalf("valid WAV indexed with duration <= 0: %v", song.Duration)
	}
	if song.CreatedAt.IsZero() {
		t.Fatal("upload 201 echoes zero-value createdAt")
	}
	after, err := st.AllSongs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before)+1 {
		t.Fatalf("catalog count wrong: before=%d after=%d", len(before), len(after))
	}
}

// testWAV returns a minimal valid PCM WAV of the given duration (seconds).
func testWAV(t *testing.T, seconds int) []byte {
	t.Helper()
	rate := 44100
	dataSize := rate * seconds * 2 // 16-bit mono
	buf := make([]byte, 44+dataSize)
	copy(buf[0:4], "RIFF")
	le32(buf, 4, uint32(36+dataSize))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	le32(buf, 16, 16)
	le16(buf, 20, 1) // PCM
	le16(buf, 22, 1) // mono
	le32(buf, 24, uint32(rate))
	le32(buf, 28, uint32(rate*2)) // byte rate
	le16(buf, 32, 2)              // block align
	le16(buf, 34, 16)             // bits per sample
	copy(buf[36:40], "data")
	le32(buf, 40, uint32(dataSize))
	return buf
}

func le16(b []byte, off, v int) { b[off] = byte(v); b[off+1] = byte(v >> 8) }
func le32(b []byte, off int, v uint32) {
	b[off] = byte(v)
	b[off+1] = byte(v >> 8)
	b[off+2] = byte(v >> 16)
	b[off+3] = byte(v >> 24)
}
