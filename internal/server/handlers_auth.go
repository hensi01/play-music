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

// handleLogin supports client phone login and the legacy administrator login.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeLoginRequest(w, r)
	if !ok {
		return
	}
	if req.Phone == "" {
		s.loginAdmin(w, r, req)
		return
	}
	if req.Username != "" || req.Password != "" {
		writeError(w, http.StatusBadRequest, "Use o acesso de clientes com seu telefone")
		return
	}

	normalized, err := phone.Normalize(req.Phone)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Telefone não cadastrado")
		return
	}
	u, token, err := s.auth.LoginPhone(r.Context(), normalized)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeLoginResponse(w, u.ID, u.Username, u.Name, u.Phone, false, token)
}

// handleAdminLogin serves POST /auth/admin/login. The legacy login route
// continues to accept administrator credentials for existing integrations.
func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeLoginRequest(w, r)
	if !ok {
		return
	}
	s.loginAdmin(w, r, req)
}

func (s *Server) loginAdmin(w http.ResponseWriter, r *http.Request, req loginRequest) {
	if req.Phone != "" || req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "Informe usuário e senha de administrador")
		return
	}

	u, token, err := s.auth.LoginUsername(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeLoginResponse(w, u.ID, u.Username, u.Name, u.Phone, true, token)
}

func decodeLoginRequest(w http.ResponseWriter, r *http.Request) (loginRequest, bool) {
	var req loginRequest
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return loginRequest{}, false
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return loginRequest{}, false
	}
	return req, true
}

func writeLoginResponse(w http.ResponseWriter, id, username, name, phone string, isAdmin bool, token string) {
	writeJSON(w, http.StatusOK, map[string]any{
		"token":    token,
		"id":       id,
		"username": username,
		"name":     name,
		"phone":    phone,
		"isAdmin":  isAdmin,
	})
}
