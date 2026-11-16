package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"play-music/internal/store"
)

type pushCampaign struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	URL         string    `json:"url"`
	ScheduledAt time.Time `json:"scheduledAt"`
	Status      string    `json:"status"`
	Selected    int       `json:"selected"`
	Queued      int       `json:"queued"`
	Detail      string    `json:"detail"`
}

func validCampaign(c *pushCampaign) error {
	c.Title = strings.TrimSpace(c.Title)
	c.Body = strings.TrimSpace(c.Body)
	c.URL = strings.TrimSpace(c.URL)
	if c.Title == "" || c.Body == "" || utf8.RuneCountInString(c.Title) > 120 || utf8.RuneCountInString(c.Body) > 500 || len(c.URL) > 1000 {
		return errors.New("Preencha título e mensagem válidos")
	}
	if c.URL == "" {
		c.URL = "/"
	}
	u, e := url.Parse(c.URL)
	if e != nil || (u.IsAbs() && (u.Scheme != "https" || u.Host == "" || u.User != nil)) || (!u.IsAbs() && (!strings.HasPrefix(c.URL, "/") || strings.HasPrefix(c.URL, "//"))) {
		return errors.New("Link deve ser HTTPS ou um caminho do aplicativo")
	}
	payload, _ := json.Marshal(map[string]any{"title": c.Title, "body": c.Body, "url": c.URL, "icon": "/icon-192.png"})
	if len(payload) > 3000 {
		return errors.New("Mensagem e link excedem o limite de tamanho")
	}
	return nil
}
func (s *Server) handleCreateCampaign(w http.ResponseWriter, r *http.Request) {
	if !s.pushEnabled() {
		writeError(w, 503, "Notificações indisponíveis")
		return
	}
	var c pushCampaign
	if !pushRead(w, r, &c) {
		return
	}
	if err := validCampaign(&c); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if c.ScheduledAt.IsZero() {
		c.ScheduledAt = time.Now().UTC()
	}
	if c.ScheduledAt.Before(time.Now().Add(-time.Minute)) {
		writeError(w, 400, "O horário não pode estar no passado")
		return
	}
	c.ID = store.NewID()
	c.Status = "pending"
	_, err := s.store.Pool().Exec(r.Context(), `INSERT INTO push_campaigns(id,created_by,title,body,url,scheduled_at,status) VALUES($1,$2,$3,$4,$5,$6,'pending')`, c.ID, userIDOf(r.Context()), c.Title, c.Body, c.URL, c.ScheduledAt)
	if err != nil {
		writeError(w, 500, "Não foi possível agendar")
		return
	}
	writeJSON(w, 201, c)
}
func (s *Server) handleListCampaigns(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.Pool().Query(r.Context(), `SELECT id,title,body,url,scheduled_at,status,selected,queued,detail FROM push_campaigns ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		writeError(w, 500, "Não foi possível consultar notificações")
		return
	}
	defer rows.Close()
	var all []pushCampaign
	for rows.Next() {
		var c pushCampaign
		if rows.Scan(&c.ID, &c.Title, &c.Body, &c.URL, &c.ScheduledAt, &c.Status, &c.Selected, &c.Queued, &c.Detail) != nil {
			writeError(w, 500, "Não foi possível consultar notificações")
			return
		}
		all = append(all, c)
	}
	if rows.Err() != nil {
		writeError(w, 500, "Não foi possível consultar notificações")
		return
	}
	writeJSON(w, 200, all)
}
func (s *Server) handleCancelCampaign(w http.ResponseWriter, r *http.Request) {
	tag, err := s.store.Pool().Exec(r.Context(), `UPDATE push_campaigns SET status='cancelled',finished_at=now() WHERE id=$1 AND status='pending'`, r.PathValue("id"))
	if err != nil {
		writeError(w, 500, "Não foi possível cancelar")
		return
	}
	if tag.RowsAffected() != 1 {
		writeError(w, 409, "Notificação não está aguardando envio")
		return
	}
	writeNoContent(w)
}

// RunPushScheduler claims each campaign once. An interrupted or ambiguous send
// is marked unknown for manual review, never automatically requeued.
func (s *Server) RunPushScheduler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			s.cleanupInactivePush(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		s.runPushCampaign(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (s *Server) runPushCampaign(ctx context.Context) {
	if !s.pushEnabled() {
		return
	}
	_, _ = s.store.Pool().Exec(ctx, `UPDATE push_campaigns SET status='unknown',detail='Envio interrompido; verifique antes de reenviar',finished_at=now() WHERE status='sending' AND started_at < now()-interval '5 minutes'`)
	var c pushCampaign
	err := s.store.Pool().QueryRow(ctx, `UPDATE push_campaigns SET status='sending',started_at=now() WHERE id=(SELECT id FROM push_campaigns WHERE status='pending' AND scheduled_at<=now() ORDER BY scheduled_at,id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id,title,body,url,scheduled_at,status`).Scan(&c.ID, &c.Title, &c.Body, &c.URL, &c.ScheduledAt, &c.Status)
	if errors.Is(err, pgx.ErrNoRows) || err != nil {
		return
	}
	work, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	rows, err := s.store.Pool().Query(work, `SELECT relay_id FROM push_subscriptions WHERE user_id IS NOT NULL AND active ORDER BY relay_id`)
	if err != nil {
		s.finishCampaign(c.ID, "unknown", 0, 0, "Não foi possível selecionar os destinatários")
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		s.finishCampaign(c.ID, "unknown", 0, 0, "Não foi possível selecionar os destinatários")
		return
	}
	selected, queued := len(ids), 0
	payload := map[string]any{"title": c.Title, "body": c.Body, "url": c.URL, "icon": "/icon-192.png"}
	for i := 0; i < len(ids); i += 100 {
		end := i + 100
		if end > len(ids) {
			end = len(ids)
		}
		var result struct {
			Data struct {
				Queued int `json:"queued"`
			} `json:"data"`
		}
		status, e := s.relay(work, "POST", "/api/send", "", map[string]any{"payload": payload, "subscriptionIds": ids[i:end]}, &result)
		if e != nil {
			state := "unknown"
			if status >= 400 && status < 500 {
				state = "partial"
			}
			s.finishCampaign(c.ID, state, selected, queued, fmt.Sprintf("Resultado do lote incompleto (HTTP %d); confira antes de reenviar", status))
			return
		}
		queued += result.Data.Queued
		_, err = s.store.Pool().Exec(work, `UPDATE push_campaigns SET selected=$2,queued=$3 WHERE id=$1 AND status='sending'`, c.ID, selected, queued)
		if err != nil {
			return
		}
	}
	s.finishCampaign(c.ID, "completed", selected, queued, "")
}
func (s *Server) finishCampaign(id, status string, selected, queued int, detail string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = s.store.Pool().Exec(ctx, `UPDATE push_campaigns SET status=$2,selected=$3,queued=$4,detail=$5,finished_at=now() WHERE id=$1 AND status='sending'`, id, status, selected, queued, detail)
}

// cleanupInactivePush keeps opt-outs disabled locally while revoking them remotely.
func (s *Server) cleanupInactivePush(ctx context.Context) {
	if !s.pushEnabled() {
		return
	}
	tx, err := s.store.Pool().Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(context.Background())
	var endpoint, token string
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT endpoint,management_token,subscription_data FROM push_subscriptions WHERE (NOT active OR user_id IS NULL) AND updated_at < now()-interval '2 minutes' ORDER BY updated_at FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&endpoint, &token, &raw)
	if err != nil {
		return
	}
	defer func() {
		_ = tx.Rollback(context.Background())
		timeout, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = s.store.Pool().Exec(timeout, `UPDATE push_subscriptions SET updated_at=now() WHERE endpoint=$1 AND (NOT active OR user_id IS NULL)`, endpoint)
	}()
	path := "/api/apps/" + url.PathEscape(s.cfg.RelayAppID)
	status, err := s.relay(ctx, "DELETE", path+"/subscriptions", token, map[string]string{"endpoint": endpoint}, nil)
	if err != nil && status == 401 {
		var sub pushSubscription
		if json.Unmarshal(raw, &sub) != nil {
			return
		}
		var ticket struct {
			Data struct {
				Token string `json:"token"`
			} `json:"data"`
		}
		_, err = s.relay(ctx, "POST", path+"/enrollment-ticket", "", sub, &ticket)
		if err != nil || ticket.Data.Token == "" {
			return
		}
		var out struct {
			Data struct {
				Management struct {
					Token string `json:"token"`
				} `json:"management"`
			} `json:"data"`
		}
		_, err = s.relay(ctx, "POST", path+"/subscriptions", ticket.Data.Token, sub, &out)
		if err != nil || out.Data.Management.Token == "" {
			return
		}
		_, err = s.relay(ctx, "DELETE", path+"/subscriptions", out.Data.Management.Token, map[string]string{"endpoint": endpoint}, nil)
	}
	if err != nil {
		return
	}
	if _, err = tx.Exec(ctx, "DELETE FROM push_subscriptions WHERE endpoint=$1 AND (NOT active OR user_id IS NULL)", endpoint); err == nil {
		_ = tx.Commit(ctx)
	}
}
