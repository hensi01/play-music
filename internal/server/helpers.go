package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"golang.org/x/crypto/bcrypt"

	"play-music/internal/auth"
	"play-music/internal/model"
	"play-music/internal/phone"
	"play-music/internal/store"
)

func bcryptPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func modelUserFromClaims(c *auth.Claims) model.User {
	return model.User{
		ID:       c.UserID,
		Username: c.Username,
		Phone:    c.Phone,
		Name:     c.Name,
		IsAdmin:  c.IsAdmin,
	}
}

// formatPhone puts a stored phone (digits only) into display form. Every
// response that includes a phone goes through here, so one form is served.
func formatPhone(u *model.User) {
	if u.Phone != "" {
		u.Phone = phone.Format(u.Phone)
	}
}

// jsonContentType is the one content type for every JSON response, including
// the queue handler, which writes stored bytes rather than an encoded value.
const jsonContentType = "application/json; charset=utf-8"

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", jsonContentType)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// decodeJSON reads a size-limited JSON body into v; false means the 400 was
// already written.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(io.LimitReader(r.Body, maxJSONBody)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return false
	}
	return true
}

const maxJSONBody = 1 << 20

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

func handleStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Não encontrado")
		return
	}
	if errors.Is(err, store.ErrForbidden) {
		writeError(w, http.StatusForbidden, "Sem permissão")
		return
	}
	if errors.Is(err, store.ErrOwnedByOther) {
		writeError(w, http.StatusConflict, "Recurso vinculado a outra conta")
		return
	}
	if errors.Is(err, store.ErrStale) {
		writeError(w, http.StatusConflict, "Operação não está mais disponível")
		return
	}
	if errors.Is(err, store.ErrNotPending) {
		writeError(w, http.StatusConflict, "Notificação não está aguardando envio")
		return
	}
	if errors.Is(err, store.ErrDuplicate) {
		writeError(w, http.StatusConflict, "Já existe um usuário com esse e-mail, usuário ou telefone")
		return
	}
	slog.Error("internal error", "err", err)
	writeError(w, http.StatusInternalServerError, "Erro interno")
}

type ctxKey string

const ctxUserID ctxKey = "user_id"

func userIDOf(ctx context.Context) string {
	if v, ok := ctx.Value(ctxUserID).(string); ok {
		return v
	}
	return ""
}
