package server

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
)

var playIDPattern = regexp.MustCompile(`^[a-zA-Z0-9-]{16,64}$`)

func (s *Server) handleQualifiedListen(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SongID string `json:"songId"`
		PlayID string `json:"playId"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil || req.SongID == "" || !playIDPattern.MatchString(req.PlayID) {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return
	}
	ok, err := s.store.SongExists(r.Context(), req.SongID)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "Música não encontrada")
		return
	}
	if err := s.store.RegisterQualifiedListen(r.Context(), userIDOf(r.Context()), req.SongID, req.PlayID); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}
func (s *Server) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	result, err := s.store.Recommendations(r.Context(), userIDOf(r.Context()))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, result)
}
