package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"play-music/internal/model"
	"play-music/internal/phone"
	"play-music/internal/store"
)

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	normalized, err := phone.Normalize(strings.TrimSpace(req.Phone))
	if err != nil {
		writeError(w, http.StatusBadRequest, phone.ErrInvalid.Error())
		return
	}

	ctx := r.Context()
	u, err := s.store.GetUserByPhone(ctx, normalized)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		handleStoreError(w, err)
		return
	}
	if u != nil {
		writeError(w, http.StatusConflict, "Telefone já cadastrado; entre na sua conta")
		return
	}
	hash, err := bcryptPassword(randomSecret())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	u = &model.User{ID: store.NewID(), Phone: normalized, Name: normalized}
	if err = s.store.CreateUser(ctx, u, hash, nil); err != nil {
		handleStoreError(w, err)
		return
	}

	token, err := s.auth.Sign(model.User{ID: u.ID, Phone: u.Phone, Name: u.Name})
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeLoginResponse(w, *u, false, token)
}

// randomSecret is the unusable password hash given to phone-only accounts: they
// authenticate by phone, never by password. A failing crypto/rand is fatal here
// for the same reason store.NewID treats it as fatal.
func randomSecret() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failure: " + err.Error())
	}
	return hex.EncodeToString(b)
}
