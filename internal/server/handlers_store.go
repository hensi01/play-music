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

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
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

	u, _, err := s.store.GetUserByPhone(ctx, normalized)
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
	if err := s.store.CreateUser(ctx, u, hash, nil); err != nil {
		handleStoreError(w, err)
		return
	}
	u.Phone = phone.Format(u.Phone)

	token, err := s.auth.Sign(ctx, model.User{ID: u.ID, Phone: u.Phone, Name: u.Name})
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "id": u.ID, "name": u.Name, "phone": u.Phone, "isAdmin": false})
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
