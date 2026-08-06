package server

import (
	"encoding/json"
	"io"
	"net/http"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleLogin serves POST /auth/login and POST /auth/createAdmin. The admin
// credentials come exclusively from the environment; createAdmin is accepted
// for compatibility with the first-time setup flow but validates the same
// credentials.
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
	user, token, err := s.auth.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":   token,
		"id":      user.ID,
		"username": user.Username,
		"name":    user.Name,
		"isAdmin": user.IsAdmin,
	})
}
