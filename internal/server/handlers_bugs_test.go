package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"play-music/internal/config"
	"play-music/internal/db"
	"play-music/internal/metadata"
	"play-music/internal/model"
	"play-music/internal/phone"
	"play-music/internal/scanner"
	"play-music/internal/storage"
	"play-music/internal/store"
)

// These tests exercise handler fixes (B2, B-NEW-2, B-NEW-3) against a real
// Postgres — the Server depends on a concrete *store.Store, so a unit-level
// mock is not possible without an invasive interface refactor. They are
// integration tests that skip cleanly when DATABASE_URL (env or repo .env)
// is unavailable, keeping `go test ./...` green in any environment.

// testDBURL resolves DATABASE_URL from the environment, falling back to the
// repo .env (the test process runs with the package dir as CWD). Skips when
// no database is available.
func testDBURL(t *testing.T) string {
	t.Helper()
	if u := os.Getenv("DATABASE_URL"); u != "" {
		return u
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Skipf("resolve repo root: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil {
		t.Skip("DATABASE_URL not set and repo .env not found — integration test skipped")
		return "" // unreachable; satisfies the vet missing-return check
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "DATABASE_URL="); ok {
			if v = strings.Trim(strings.TrimSpace(v), `"'`); v != "" {
				return v
			}
		}
	}
	t.Skip("DATABASE_URL not set in environment or repo .env — integration test skipped")
	return "" // unreachable; satisfies the vet missing-return check
}

func newTestServerStore(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	pool, err := db.Connect(context.Background(), testDBURL(t))
	if err != nil {
		t.Skipf("postgres unreachable: %v", err)
		return nil, nil // unreachable; satisfies the vet missing-return check
	}
	t.Cleanup(pool.Close)
	st := store.New(pool)
	return &Server{store: st, log: slog.Default()}, st
}

// loadDotEnv loads the repo .env into the process environment for keys that
// are not already set (config.Load reads only from os.Getenv).
func loadDotEnv(t *testing.T) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Skipf("resolve repo root: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil {
		t.Skipf("repo .env not found: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if os.Getenv(k) == "" && v != "" {
			os.Setenv(k, v)
		}
	}
}

// newFullTestServer builds a Server with the real wiring (store + storage +
// scanner) so upload happy paths can be exercised end-to-end. Skips when the
// database or the S3/MinIO config is unavailable.
func newFullTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	loadDotEnv(t)
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		t.Skip("DATABASE_URL not set — integration test skipped")
	}
	if cfg.S3.Endpoint == "" || cfg.S3.AccessKey == "" || cfg.S3.SecretKey == "" {
		t.Skip("S3/MinIO config missing — integration test skipped")
	}
	pool, err := db.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		t.Skipf("postgres unreachable: %v", err)
		return nil, nil // unreachable; satisfies the vet missing-return check
	}
	t.Cleanup(pool.Close)
	st := store.New(pool)
	strg, err := storage.New(cfg.S3)
	if err != nil {
		t.Skipf("storage init: %v", err)
		return nil, nil // unreachable; satisfies the vet missing-return check
	}
	metadata.SetFFmpegPath(cfg.FfmpegPath)
	sc := scanner.New(cfg, strg, st, slog.Default())
	return &Server{store: st, storage: strg, scanner: sc, log: slog.Default()}, st
}

// randomDigits returns n uniform decimal digits (test phone numbers).
func randomDigits(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteByte(byte('0' + b[i]%10))
	}
	return sb.String()
}

// B2: POST /api/admin/users used to echo createdAt "0001-01-01T00:00:00Z"
// (zero-value) because the INSERT computed now() but never returned it.
// The store now does RETURNING created_at; the handler response must carry
// a real timestamp and the JSON must not serialize the zero-value.
func TestAdminCreateUserReturnsCreatedAt(t *testing.T) {
	s, st := newTestServerStore(t)

	var (
		u   model.User
		rec *httptest.ResponseRecorder
	)
	for i := 0; i < 3; i++ {
		phone := "1" + randomDigits(9)
		body := `{"name":"Test Worker","phone":"` + phone + `"}`
		rec = httptest.NewRecorder()
		s.handleAdminCreateUser(rec,
			httptest.NewRequest(http.MethodPost, "/api/admin/users", strings.NewReader(body)))
		if rec.Code == http.StatusConflict { // phone collision: retry
			continue
		}
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &u); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		break
	}
	if u.ID == "" {
		t.Fatal("could not create a test user (phone collisions)")
	}
	t.Cleanup(func() { _ = st.DeleteUser(context.Background(), u.ID) })

	if u.CreatedAt.IsZero() {
		t.Fatal("B2 not fixed: user.CreatedAt is zero-value after create")
	}
	if u.CreatedAt.Year() < 2020 {
		t.Fatalf("createdAt looks wrong: %v", u.CreatedAt)
	}
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if raw["createdAt"] == "0001-01-01T00:00:00Z" {
		t.Fatal("B2 not fixed: createdAt still serialized as zero-value")
	}
	if v, _ := raw["createdAt"].(string); v == "" {
		t.Fatal("B2 not fixed: createdAt missing from JSON response")
	}
}

// B-NEW-3: liking/unliking a song that does not exist used to return 204
// silently, desyncing client state. It must now answer 404.
func TestLikeUnknownSongReturns404(t *testing.T) {
	s, _ := newTestServerStore(t)
	ctx := context.WithValue(context.Background(), ctxUserID, store.NewID())
	unknown := store.NewID()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/me/liked/"+unknown, nil).WithContext(ctx)
	req.SetPathValue("id", unknown) // handleLike reads r.PathValue("id")
	s.handleLike(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("like unknown: status = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/me/liked/"+unknown, nil).WithContext(ctx)
	req.SetPathValue("id", unknown)
	s.handleUnlike(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unlike unknown: status = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}
}

// Regression guard for the B-NEW-3 validation: a real song id must still
// like/unlike with 204.
func TestLikeExistingSongReturns204(t *testing.T) {
	s, st := newTestServerStore(t)
	var songID string
	if err := st.Pool().QueryRow(context.Background(),
		"SELECT id FROM songs LIMIT 1").Scan(&songID); err != nil {
		t.Skipf("no songs in the catalog — skipping: %v", err)
	}
	ctx := context.WithValue(context.Background(), ctxUserID, store.NewID())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/me/liked/"+songID, nil).WithContext(ctx)
	req.SetPathValue("id", songID)
	s.handleLike(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("like existing: status = %d, want 204 (body %s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/me/liked/"+songID, nil).WithContext(ctx)
	req.SetPathValue("id", songID)
	s.handleUnlike(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("unlike existing: status = %d, want 204 (body %s)", rec.Code, rec.Body.String())
	}
}

// B-NEW-2: /api/store/register used to silently ignore unknown category ids
// (conceding nothing but also reporting success). It must now reject the
// request with 400 listing the invalid ids, and must NOT create the user.
func TestStoreRegisterRejectsUnknownCategories(t *testing.T) {
	s, st := newTestServerStore(t)
	p := "1" + randomDigits(9)
	body := `{"phone":"` + p + `","categoryIds":["does-not-exist-1","also-invalid"]}`

	rec := httptest.NewRecorder()
	s.handleStoreRegister(rec,
		httptest.NewRequest(http.MethodPost, "/api/store/register", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	invalid, ok := resp["invalidCategoryIds"].([]any)
	if !ok || len(invalid) != 2 {
		t.Fatalf("invalidCategoryIds missing/wrong: %v", resp)
	}

	// The request must not have created the user (validation runs first).
	normalized, err := phone.Normalize(p) // "1"+9 digits: always valid
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.GetUserByPhone(context.Background(), normalized); err == nil {
		t.Fatal("B-NEW-2: user was created even though the request was rejected")
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

// Upload validation: a file renamed to .mp3 whose content is not decodable
// audio (junk bytes, no metadata, no duration) used to be accepted with a
// 201 and indexed with duration 0. It must now be rejected with 400 and the
// message must state the file is invalid — and nothing may be inserted.
func TestAdminUploadRejectsInvalidAudio(t *testing.T) {
	s, st := newTestServerStore(t)
	before := countSongs(t, st)

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
	if msg, _ := resp["error"].(string); !strings.Contains(msg, "Áudio inválido") && !strings.Contains(msg, "áudio inválido") {
		t.Fatalf("error message does not mention invalid audio: %q", msg)
	}
	if after := countSongs(t, st); after != before {
		t.Fatalf("catalog changed by the rejected upload: before=%d after=%d", before, after)
	}
}

// A valid untagged WAV (RIFF/PCM) must pass the pre-Put validation and be
// indexed with a real duration (>0), never 0. Requires the full
// server wiring (storage + scanner); skipped when those are unavailable.
func TestAdminUploadValidWavHasDurationAndCreatedAt(t *testing.T) {
	s, st := newFullTestServer(t)
	before := countSongs(t, st)

	var songID string
	defer func() {
		if songID != "" {
			if _, err := st.Pool().Exec(context.Background(), "DELETE FROM songs WHERE id=$1", songID); err != nil {
				t.Logf("cleanup of uploaded WAV song: %v", err)
			}
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
	songID = song.ID
	if song.Duration <= 0 {
		t.Fatalf("valid WAV indexed with duration <= 0: %v", song.Duration)
	}
	if song.CreatedAt.IsZero() {
		t.Fatal("upload 201 echoes zero-value createdAt (fix 2 not applied)")
	}
	if after := countSongs(t, st); after != before+1 {
		t.Fatalf("catalog count wrong: before=%d after=%d", before, after)
	}
}

func countSongs(t *testing.T, st *store.Store) int {
	t.Helper()
	var n int
	if err := st.Pool().QueryRow(context.Background(), "SELECT count(*) FROM songs").Scan(&n); err != nil {
		t.Skipf("cannot count songs: %v", err)
	}
	return n
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
