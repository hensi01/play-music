package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"play-music/internal/model"
	"play-music/internal/phone"
	"play-music/internal/store"
)

// ---------- store: user registration / category releases ----------

// handleStoreCategoryPhoto serves the category cover publicly (store page, no
// auth). Categories are listed publicly by the store, so their covers are
// public too. Reuses the artwork pipeline (custom photo -> placeholder) and
// its cache; the id never reaches the access-guarded chain, so no content is
// exposed beyond the category cover itself.
func (s *Server) handleStoreCategoryPhoto(w http.ResponseWriter, r *http.Request) {
	size := parseIntQuery(r, "size", 300)
	s.artwork.Serve(w, r, r.PathValue("id"), size)
}

// handleStoreCategories lists all categories publicly (store page, no auth).
func (s *Server) handleStoreCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := s.store.GetCategories(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cats)
}

// handleStoreRegister creates a client user using only the phone number and
// grants the given categories. Used after an external checkout (payment) or
// for manual releases. Returns the created/updated user with a fresh token
// (auto-login).
//
// TODO(security): this endpoint is public and grants PAID categories without
// proof of payment — the external checkout is not integrated yet and the
// store UI depends on this flow. When the payment gateway is wired in,
// verify the payment/webhook before granting categories and only grant the
// ids present in the paid order. The validation below (unknown category ids
// rejected with 400) is a minimal mitigation, not a payment check.
func (s *Server) handleStoreRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone       string   `json:"phone"`
		CategoryIDs []string `json:"categoryIds"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return
	}
	normalized, err := phone.Normalize(strings.TrimSpace(req.Phone))
	if err != nil {
		writeError(w, http.StatusBadRequest, phone.ErrInvalid.Error())
		return
	}

	ctx := r.Context()

	// Reject unknown category ids with a 400 listing them, instead of
	// silently dropping them (a silent drop would make the client believe a
	// category was granted). The store UI sends categoryIds:[] for plain
	// registrations, so an empty list is valid and yields no error. This
	// runs BEFORE any user creation so a rejected request leaves no state.
	valid, err := s.store.GetCategories(ctx)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	validIDs := make(map[string]bool, len(valid))
	for _, c := range valid {
		validIDs[c.ID] = true
	}
	var invalid []string
	for _, cid := range req.CategoryIDs {
		if cid != "" && !validIDs[cid] {
			invalid = append(invalid, cid)
		}
	}
	if len(invalid) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":              "Categorias inexistentes",
			"invalidCategoryIds": invalid,
		})
		return
	}

	u, _, err := s.store.GetUserByPhone(ctx, normalized)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		handleStoreError(w, err)
		return
	}
	if u == nil {
		hash, err := bcryptPassword(randomSecret())
		if err != nil {
			handleStoreError(w, err)
			return
		}
		u = &model.User{ID: store.NewID(), Phone: normalized, Name: normalized, IsAdmin: false}
		if err := s.store.CreateUser(ctx, u, hash, nil); err != nil {
			handleStoreError(w, err)
			return
		}
	} else if u.IsAdmin {
		writeError(w, http.StatusBadRequest, "Este telefone pertence a uma conta de administrador")
		return
	}
	if err := s.store.GrantUserCategories(ctx, u.ID, req.CategoryIDs); err != nil {
		handleStoreError(w, err)
		return
	}

	u.Categories, err = s.store.UserCategories(ctx, u.ID)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	u.Phone = phone.Format(u.Phone)

	token, err := s.auth.Sign(ctx, model.User{ID: u.ID, Phone: u.Phone, Name: u.Name})
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": u})
}

// handleStorePurchase grants categories to the authenticated user (a client
// buying/releasing categories).
func (s *Server) handleStorePurchase(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CategoryIDs []string `json:"categoryIds"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return
	}
	if err := s.store.GrantUserCategories(r.Context(), userIDOf(r.Context()), req.CategoryIDs); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

// randomSecret returns a random hex string (random password for phone-only
// accounts, which never log in with a password).
func randomSecret() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "play-music"
	}
	return hex.EncodeToString(b)
}
