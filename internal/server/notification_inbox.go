package server

import (
	"net/http"
)

func (s *Server) handleNotificationInbox(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.NotificationInbox(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
