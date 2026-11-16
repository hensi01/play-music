package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"play-music/internal/store"
)

// requireAdminAPIKey checks an integration key against its current owner.
func (s *Server) requireAdminAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		if !strings.HasPrefix(key, "pmk_") || len(key) != 47 {
			writeError(w, 401, "Chave de API inválida")
			return
		}
		if _, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(key, "pmk_")); err != nil {
			writeError(w, 401, "Chave de API inválida")
			return
		}
		hash := sha256.Sum256([]byte(key))
		var id, user string
		err := s.store.Pool().QueryRow(r.Context(), `UPDATE api_integration_keys k SET last_used_at=now() FROM users u WHERE k.key_hash=$1 AND k.revoked_at IS NULL AND u.id=k.owner_id AND u.is_admin=true AND EXISTS (SELECT 1 FROM push_subscriptions ps WHERE ps.user_id=u.id AND ps.active) RETURNING k.id,k.owner_id`, hash[:]).Scan(&id, &user)
		if err != nil {
			if err == pgx.ErrNoRows {
				writeError(w, 401, "Chave de API inválida")
			} else {
				writeError(w, 503, "Não foi possível verificar a chave")
			}
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserID, user)
		ctx = context.WithValue(ctx, ctxIsAdmin, true)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, 400, "Nome inválido")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 80 {
		writeError(w, 400, "Informe um nome com até 80 caracteres")
		return
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		writeError(w, 500, "Não foi possível criar a chave")
		return
	}
	key := "pmk_" + base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(key))
	id := store.NewID()
	_, err := s.store.Pool().Exec(r.Context(), `INSERT INTO api_integration_keys(id,name,key_hash,owner_id) VALUES($1,$2,$3,$4)`, id, req.Name, hash[:], userIDOf(r.Context()))
	if err != nil {
		writeError(w, 500, "Não foi possível criar a chave")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 201, map[string]string{"id": id, "name": req.Name, "key": key})
}

type apiKeyInfo struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.Pool().Query(r.Context(), `SELECT id,name,created_at,last_used_at FROM api_integration_keys WHERE revoked_at IS NULL ORDER BY created_at DESC`)
	if err != nil {
		writeError(w, 500, "Não foi possível listar as chaves")
		return
	}
	defer rows.Close()
	all := []apiKeyInfo{}
	for rows.Next() {
		var key apiKeyInfo
		if rows.Scan(&key.ID, &key.Name, &key.CreatedAt, &key.LastUsedAt) != nil {
			writeError(w, 500, "Não foi possível listar as chaves")
			return
		}
		all = append(all, key)
	}
	if rows.Err() != nil {
		writeError(w, 500, "Não foi possível listar as chaves")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, all)
}
func (s *Server) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	tag, err := s.store.Pool().Exec(r.Context(), `UPDATE api_integration_keys SET revoked_at=now() WHERE id=$1 AND revoked_at IS NULL`, r.PathValue("id"))
	if err != nil {
		writeError(w, 500, "Não foi possível revogar a chave")
		return
	}
	if tag.RowsAffected() != 1 {
		writeError(w, 404, "Chave não encontrada")
		return
	}
	writeNoContent(w)
}
