package server

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"play-music/internal/model"
	"play-music/internal/store"
)

// multipartKaraoke builds a multipart/form-data body with a single "video" file.
func multipartKaraoke(t *testing.T, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("video", filename)
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

// Upload validation: a file renamed to .mp4 whose content is not decodable
// video (junk bytes, no duration) must be rejected with 400 before touching
// the bucket — and nothing may be inserted.
func TestAdminUploadKaraokeRejectsInvalidVideo(t *testing.T) {
	s, st := newTestServerStore(t)
	before := countKaraokes(t, st)

	body, contentType := multipartKaraoke(t, "fake.mp4", bytes.Repeat([]byte("x"), 1024))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/karaokes", body)
	req.Header.Set("Content-Type", contentType)
	s.handleAdminUploadKaraoke(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if msg, _ := resp["error"].(string); !strings.Contains(msg, "vídeo inválido") {
		t.Fatalf("error message does not mention invalid video: %q", msg)
	}
	if after := countKaraokes(t, st); after != before {
		t.Fatalf("catalog changed by the rejected upload: before=%d after=%d", before, after)
	}
}

// A non-video extension must be rejected by the extension whitelist.
func TestAdminUploadKaraokeRejectsBadExtension(t *testing.T) {
	s, st := newTestServerStore(t)
	before := countKaraokes(t, st)

	body, contentType := multipartKaraoke(t, "clip.avi", bytes.Repeat([]byte("x"), 1024))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/karaokes", body)
	req.Header.Set("Content-Type", contentType)
	s.handleAdminUploadKaraoke(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
	if after := countKaraokes(t, st); after != before {
		t.Fatalf("catalog changed by the rejected upload: before=%d after=%d", before, after)
	}
}

// Access control: karaokes are only visible to users whose granted categories
// contain them; admins (userID "") see everything.
func TestKaraokeAccessByCategory(t *testing.T) {
	_, st := newTestServerStore(t)
	ctx := context.Background()

	cat, err := st.CreateCategory(ctx, "Karaokê Teste", "")
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	clientID := store.NewID()
	otherID := store.NewID()
	client := &model.User{ID: clientID, Phone: "1" + randomDigits(9), Name: "Karaoke Client"}
	if err := st.CreateUser(ctx, client, "x", nil); err != nil {
		t.Fatalf("create user: %v", err)
	}
	other := &model.User{ID: otherID, Phone: "1" + randomDigits(9), Name: "No Access"}
	if err := st.CreateUser(ctx, other, "x", nil); err != nil {
		t.Fatalf("create other user: %v", err)
	}
	t.Cleanup(func() {
		st.Pool().Exec(ctx, "DELETE FROM users WHERE id=$1 OR id=$2", client.ID, other.ID)
		st.Pool().Exec(ctx, "DELETE FROM categories WHERE id=$1", cat.ID)
	})

	karaokeID := store.NewID()
	if _, err := st.Pool().Exec(ctx, `
		INSERT INTO karaokes (id, path, title, duration, format, created_at, updated_at)
		VALUES ($1, 'uploads/test.mp4', 'Teste', 180, 'mp4', now(), now())`, karaokeID); err != nil {
		t.Fatalf("insert karaoke: %v", err)
	}
	t.Cleanup(func() {
		st.Pool().Exec(ctx, "DELETE FROM karaokes WHERE id=$1", karaokeID)
	})
	if err := st.AddKaraokeToCategory(ctx, cat.ID, karaokeID); err != nil {
		t.Fatalf("assign karaoke: %v", err)
	}

	// Without the category granted: no access, karaoke absent from the list.
	ok, err := st.CanAccessKaraoke(ctx, other.ID, karaokeID)
	if err != nil || ok {
		t.Fatalf("no-grant user access = %v, %v; want false", ok, err)
	}
	list, err := st.AllKaraokes(ctx, other.ID)
	if err != nil || containsKaraoke(list, karaokeID) {
		t.Fatalf("no-grant user list must not contain the karaoke: %v (%v)", list, err)
	}

	// After granting the category: access, list contains the karaoke.
	if err := st.GrantUserCategories(ctx, client.ID, []string{cat.ID}); err != nil {
		t.Fatalf("grant category: %v", err)
	}
	ok, err = st.CanAccessKaraoke(ctx, client.ID, karaokeID)
	if err != nil || !ok {
		t.Fatalf("granted user access = %v, %v; want true", ok, err)
	}
	list, err = st.AllKaraokes(ctx, client.ID)
	if err != nil || !containsKaraoke(list, karaokeID) {
		t.Fatalf("granted user list must contain the karaoke: %v (%v)", list, err)
	}

	// Admin sees everything.
	list, err = st.AllKaraokes(ctx, "")
	if err != nil || !containsKaraoke(list, karaokeID) {
		t.Fatalf("admin list must contain the karaoke: %v (%v)", list, err)
	}

	// Detail + category association round-trip.
	k, err := st.GetKaraoke(ctx, karaokeID)
	if err != nil || k == nil || k.Title != "Teste" || k.Duration != 180 {
		t.Fatalf("get karaoke: %v (%v)", k, err)
	}
	ck, err := st.CategoryKaraokes(ctx, cat.ID)
	if err != nil || len(ck) != 1 {
		t.Fatalf("category karaokes: %v (%v)", ck, err)
	}
	all, err := st.KaraokeCategoryIDs(ctx)
	if err != nil || len(all) != 1 || len(all[karaokeID]) != 1 || all[karaokeID][0] != cat.ID {
		t.Fatalf("karaoke category ids: %v (%v)", all, err)
	}

	// RegisterPlay with no access must fail; with access it must bump the count.
	if err := st.RegisterKaraokePlay(ctx, other.ID, other.ID, karaokeID); err != store.ErrForbidden {
		t.Fatalf("no-grant play error = %v, want ErrForbidden", err)
	}
	if err := st.RegisterKaraokePlay(ctx, client.ID, client.ID, karaokeID); err != nil {
		t.Fatalf("granted play: %v", err)
	}
	k2, err := st.GetKaraoke(ctx, karaokeID)
	if err != nil || k2.PlayCount != 1 {
		t.Fatalf("play count = %v (%v), want 1", k2.PlayCount, err)
	}

	// Category assignment update: karaokeIds replace the set.
	if err := st.UpdateCategory(ctx, cat.ID, "", nil, nil, []string{}); err != nil {
		t.Fatalf("update category (clear karaokes): %v", err)
	}
	ck, err = st.CategoryKaraokes(ctx, cat.ID)
	if err != nil || len(ck) != 0 {
		t.Fatalf("cleared category karaokes: %v (%v)", ck, err)
	}
}

func countKaraokes(t *testing.T, st *store.Store) int {
	t.Helper()
	var n int
	if err := st.Pool().QueryRow(context.Background(), "SELECT count(*) FROM karaokes").Scan(&n); err != nil {
		t.Skipf("cannot count karaokes: %v", err)
	}
	return n
}

// containsKaraoke reports whether the list contains the karaoke id.
func containsKaraoke(list []model.Karaoke, id string) bool {
	for _, k := range list {
		if k.ID == id {
			return true
		}
	}
	return false
}
