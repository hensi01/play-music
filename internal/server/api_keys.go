package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"play-music/internal/store"
)

const (
	apiKeyHeader = "X-API-Key"
	apiKeyPrefix = "pmk_"
	// apiKeyBody is the random part of a key, in bytes.
	apiKeyBody = 32
)

// A key is the prefix plus the body in unpadded base64url.
var apiKeyLen = len(apiKeyPrefix) + base64.RawURLEncoding.EncodedLen(apiKeyBody)

// errNotOurKey is the answer for a key that cannot have been minted here. It is
// distinct from a store failure, which is a 503 rather than a 401.
var errNotOurKey = errors.New("not an issued api key")

func (s *Server) requireAdminAPIKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owner, err := s.apiKeyOwner(r)
		switch {
		case errors.Is(err, errNotOurKey):
			writeError(w, http.StatusUnauthorized, "Chave de API inválida")
			return
		case err != nil:
			writeError(w, http.StatusServiceUnavailable, "Não foi possível verificar a chave")
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserID, owner)
		next(w, r.WithContext(ctx))
	}
}

// apiKeyOwner resolves the admin an integration key belongs to. A key whose
// shape is wrong cannot have been minted here, so it is rejected before the
// store is asked; a store failure is a different answer (503, not 401).
func (s *Server) apiKeyOwner(r *http.Request) (string, error) {
	key := r.Header.Get(apiKeyHeader)
	if !strings.HasPrefix(key, apiKeyPrefix) || len(key) != apiKeyLen {
		return "", errNotOurKey
	}
	if _, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(key, apiKeyPrefix)); err != nil {
		return "", errNotOurKey
	}
	hash := sha256.Sum256([]byte(key))
	owner, err := s.store.TouchAPIKey(r.Context(), hash[:])
	if errors.Is(err, store.ErrNotFound) {
		return "", errNotOurKey
	}
	if err != nil {
		return "", err
	}
	return owner, nil
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 80 {
		writeError(w, http.StatusBadRequest, "Informe um nome com até 80 caracteres")
		return
	}
	raw := make([]byte, apiKeyBody)
	if _, err := rand.Read(raw); err != nil {
		writeError(w, http.StatusInternalServerError, "Não foi possível criar a chave")
		return
	}
	key := apiKeyPrefix + base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(key))
	id := store.NewID()
	if err := s.store.CreateAPIKey(r.Context(), id, req.Name, hash[:], userIDOf(r.Context())); err != nil {
		writeError(w, http.StatusInternalServerError, "Não foi possível criar a chave")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "name": req.Name, "key": key})
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.store.ListAPIKeys(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Não foi possível listar as chaves")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, keys)
}

func (s *Server) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	if err := s.store.RevokeAPIKey(r.Context(), r.PathValue("id")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Chave não encontrada")
			return
		}
		writeError(w, http.StatusInternalServerError, "Não foi possível revogar a chave")
		return
	}
	writeNoContent(w)
}
