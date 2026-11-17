package server

import (
	"net/http"
	"time"
)

type inboxNotification struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"publishedAt"`
}

func (s *Server) handleNotificationInbox(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.Pool().Query(r.Context(), `SELECT id,title,body,url,published_at FROM notification_inbox ORDER BY published_at DESC,id DESC LIMIT 100`)
	if err != nil {
		writeError(w, 500, "Não foi possível consultar notificações")
		return
	}
	defer rows.Close()
	result := make([]inboxNotification, 0)
	for rows.Next() {
		var item inboxNotification
		if err = rows.Scan(&item.ID, &item.Title, &item.Body, &item.URL, &item.PublishedAt); err != nil {
			writeError(w, 500, "Não foi possível consultar notificações")
			return
		}
		result = append(result, item)
	}
	if rows.Err() != nil {
		writeError(w, 500, "Não foi possível consultar notificações")
		return
	}
	writeJSON(w, 200, result)
}
