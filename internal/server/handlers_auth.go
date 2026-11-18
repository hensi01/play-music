package server

import (
	"net/http"

	"play-music/internal/model"
	"play-music/internal/phone"
)

type loginRequest struct {
	Username string `json:"username"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

// handleLogin serves POST /auth/login: phone login for clients, and the legacy
// administrator login when only username/password are sent.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
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
	writeLoginResponse(w, u, false, token)
}

func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
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
	writeLoginResponse(w, u, true, token)
}

// session is the identity a login or registration returns: the token plus the
// fields the client needs to start a session.
type session struct {
	Token    string `json:"token"`
	ID       string `json:"id"`
	Username string `json:"username,omitempty"`
	Name     string `json:"name"`
	Phone    string `json:"phone,omitempty"`
	IsAdmin  bool   `json:"isAdmin"`
}

func writeLoginResponse(w http.ResponseWriter, u model.User, isAdmin bool, token string) {
	formatPhone(&u)
	writeJSON(w, http.StatusOK, session{
		Token:    token,
		ID:       u.ID,
		Username: u.Username,
		Name:     u.Name,
		Phone:    u.Phone,
		IsAdmin:  isAdmin,
	})
}
