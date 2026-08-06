package server

import (
	"encoding/json"
	"io"
	"net/http"

	"play-music/internal/phone"
)

type loginRequest struct {
	Username string `json:"username"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

// handleLogin serves POST /auth/login. Clients authenticate with {phone}
// ONLY (no password — access comes from the account the admin created).
// Administrators use {username, password}.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return
	}

	var user = struct {
		id       string
		name     string
		phone    string
		username string
		isAdmin  bool
	}{}
	var token string

	if req.Phone != "" {
		// Client login: phone only.
		normalized, err := phone.Normalize(req.Phone)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Telefone não cadastrado")
			return
		}
		u, tok, err := s.auth.LoginPhone(r.Context(), normalized)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		user.id, user.name, user.phone, user.isAdmin = u.ID, u.Name, u.Phone, u.IsAdmin
		token = tok
	} else {
		// Admin login: username + password.
		if req.Username == "" || req.Password == "" {
			writeError(w, http.StatusBadRequest, "Informe usuário e senha")
			return
		}
		u, tok, err := s.auth.LoginUsername(r.Context(), req.Username, req.Password)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		user.id, user.name, user.username, user.isAdmin = u.ID, u.Name, u.Username, u.IsAdmin
		token = tok
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":    token,
		"id":       user.id,
		"username": user.username,
		"name":     user.name,
		"phone":    user.phone,
		"isAdmin":  user.isAdmin,
	})
}
