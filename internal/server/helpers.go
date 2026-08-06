package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"golang.org/x/crypto/bcrypt"

	"play-music/internal/auth"
	"play-music/internal/model"
	"play-music/internal/store"
)

// bcryptPassword hashes a plain password.
func bcryptPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// modelUserFromClaims rebuilds a user for token refresh.
func modelUserFromClaims(c *auth.Claims) model.User {
	return model.User{
		ID:       c.UserID,
		Username: c.Username,
		Phone:    c.Phone,
		Name:     c.Name,
		IsAdmin:  c.IsAdmin,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func parseID(r *http.Request) string {
	return r.PathValue("id")
}

func parseIntQuery(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// handleStoreError maps store errors to HTTP responses.
func handleStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Não encontrado")
		return
	}
	if errors.Is(err, store.ErrForbidden) {
		writeError(w, http.StatusForbidden, "Sem permissão")
		return
	}
	slog.Error("internal error", "err", err)
	writeError(w, http.StatusInternalServerError, "Erro interno")
}

// ---------- request context ----------

type ctxKey string

const (
	ctxUserID ctxKey = "user_id"
	ctxIsAdmin ctxKey = "is_admin"
)

func userIDOf(ctx context.Context) string {
	if v, ok := ctx.Value(ctxUserID).(string); ok {
		return v
	}
	return ""
}

func isAdminOf(ctx context.Context) bool {
	v, _ := ctx.Value(ctxIsAdmin).(bool)
	return v
}

// filterUser returns the user id for ACCESS FILTERS: "" (see everything) for
// admins, the user id otherwise. Owner-scoped operations (playlists, likes,
// history, queue) must keep using userIDOf(ctx).
func (s *Server) filterUser(r *http.Request) string {
	if isAdminOf(r.Context()) {
		return ""
	}
	return userIDOf(r.Context())
}
