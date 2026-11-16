package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"play-music/internal/store"
)

var pushHTTP = &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func (s *Server) pushEnabled() bool {
	c := s.cfg
	return c.PushProvider == "relay" && c.RelayAppID != "" && c.RelayAppSecret != "" && c.RelayVapidPublicKey != ""
}
func (s *Server) relay(ctx context.Context, method, path, auth string, body any, result any) (int, error) {
	base := s.cfg.RelayBaseURL
	if base == "" {
		base = "https://vapid.party"
	}
	u, e := url.Parse(base)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return 0, errors.New("invalid relay URL")
	}
	b, e := json.Marshal(body)
	if e != nil {
		return 0, e
	}
	req, e := http.NewRequestWithContext(ctx, method, strings.TrimRight(base, "/")+path, bytes.NewReader(b))
	if e != nil {
		return 0, e
	}
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	} else {
		req.Header.Set("X-API-Key", s.cfg.RelayAppSecret)
	}
	resp, e := pushHTTP.Do(req)
	if e != nil {
		return 0, e
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, errors.New("relay rejected request")
	}
	if result != nil {
		e = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(result)
	}
	return resp.StatusCode, e
}

// Account-level protection complements the device permission check in the UI.
func (s *Server) pushAccessAllowed(w http.ResponseWriter, r *http.Request, userID string) bool {
	if r.URL.Path == "/api/me" || strings.HasPrefix(r.URL.Path, "/api/push/") {
		return true
	}
	var enabled bool
	err := s.store.Pool().QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM push_subscriptions WHERE user_id=$1 AND active)", userID).Scan(&enabled)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "Não foi possível verificar notificações")
		return false
	}
	if !enabled {
		writeError(w, http.StatusPreconditionRequired, "Ative as notificações para usar o aplicativo")
		return false
	}
	return true
}
func (s *Server) handlePushStatus(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Endpoint string `json:"endpoint"`
	}
	if !pushRead(w, r, &v) {
		return
	}
	var enabled bool
	err := s.store.Pool().QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM push_subscriptions WHERE endpoint=$1 AND user_id=$2 AND active)", v.Endpoint, userIDOf(r.Context())).Scan(&enabled)
	if err != nil {
		writeError(w, 503, "Não foi possível verificar notificações")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]bool{"active": enabled})
}

func (s *Server) handlePushConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"enabled": s.pushEnabled(), "publicKey": s.cfg.RelayVapidPublicKey})
}
func pushRead(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	if json.NewDecoder(r.Body).Decode(v) != nil {
		writeError(w, 400, "Dados inválidos")
		return false
	}
	return true
}

type pushSubscription struct {
	Endpoint       string `json:"endpoint"`
	ExpirationTime *int64 `json:"expirationTime"`
	Keys           struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func (s *Server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	if !s.pushEnabled() {
		writeError(w, 503, "Notificações indisponíveis")
		return
	}
	var sub pushSubscription
	if !pushRead(w, r, &sub) {
		return
	}
	if len(sub.Endpoint) > 4096 || !strings.HasPrefix(sub.Endpoint, "https://") || sub.Keys.P256dh == "" || sub.Keys.Auth == "" {
		writeError(w, 400, "Inscrição inválida")
		return
	}
	uid := userIDOf(r.Context())
	tx, err := s.store.Pool().Begin(r.Context())
	if err != nil {
		writeError(w, 500, "Nao foi possivel iniciar inscricao")
		return
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", sub.Endpoint); err != nil {
		writeError(w, 500, "Nao foi possivel bloquear inscricao")
		return
	}
	var owner *string
	err = tx.QueryRow(r.Context(), "SELECT user_id FROM push_subscriptions WHERE endpoint=$1 FOR UPDATE", sub.Endpoint).Scan(&owner)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 500, "Não foi possível verificar a inscrição")
		return
	}
	if err == nil && owner != nil && *owner != uid {
		writeError(w, 409, "Este dispositivo está vinculado a outra conta; desative as notificações nessa conta primeiro")
		return
	}
	// Persist an inactive recovery record before contacting the relay. If the
	// response is lost, cleanup can recover a token from the stored subscription.
	operationID := store.NewID()
	_, err = tx.Exec(r.Context(), `INSERT INTO push_subscriptions(endpoint,user_id,relay_id,management_token,subscription_data,active,operation_id) VALUES($1,$2,$3,'',$4,false,$5) ON CONFLICT(endpoint) DO UPDATE SET user_id=excluded.user_id, subscription_data=excluded.subscription_data, active=false, operation_id=excluded.operation_id, updated_at=now() WHERE push_subscriptions.user_id=excluded.user_id OR push_subscriptions.user_id IS NULL`, sub.Endpoint, uid, "pending-"+store.NewID(), mustMarshalSubscription(sub), operationID)
	if err != nil {
		writeError(w, 500, "Não foi possível preparar a inscrição")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "Não foi possível preparar a inscrição")
		return
	}
	tx, err = s.store.Pool().Begin(r.Context())
	if err != nil {
		writeError(w, 500, "Não foi possível iniciar a inscrição")
		return
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", sub.Endpoint); err != nil {
		writeError(w, 500, "Não foi possível iniciar a inscrição")
		return
	}
	var currentOwner *string
	var currentOperation string
	if err = tx.QueryRow(r.Context(), "SELECT user_id,operation_id FROM push_subscriptions WHERE endpoint=$1 FOR UPDATE", sub.Endpoint).Scan(&currentOwner, &currentOperation); err != nil || currentOwner == nil || *currentOwner != uid || currentOperation != operationID {
		writeError(w, 409, "Inscrição não está mais disponível")
		return
	}
	path := "/api/apps/" + url.PathEscape(s.cfg.RelayAppID)
	var ticket struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	status, err := s.relay(r.Context(), "POST", path+"/enrollment-ticket", "", sub, &ticket)
	if err != nil || ticket.Data.Token == "" {
		pushRelayError(w, status)
		return
	}
	var result struct {
		Data struct {
			ID         string `json:"subscriptionId"`
			Management struct {
				Token string `json:"token"`
			} `json:"management"`
		} `json:"data"`
	}
	status, err = s.relay(r.Context(), "POST", path+"/subscriptions", ticket.Data.Token, sub, &result)
	if err != nil || result.Data.ID == "" || result.Data.Management.Token == "" {
		pushRelayError(w, status)
		return
	}
	tag, err := tx.Exec(r.Context(), `INSERT INTO push_subscriptions(endpoint,user_id,relay_id,management_token,subscription_data,active) VALUES($1,$2,$3,$4,$5,true) ON CONFLICT(endpoint) DO UPDATE SET relay_id=excluded.relay_id, management_token=excluded.management_token, subscription_data=excluded.subscription_data, active=true, updated_at=now() WHERE push_subscriptions.user_id=excluded.user_id OR push_subscriptions.user_id IS NULL`, sub.Endpoint, uid, result.Data.ID, result.Data.Management.Token, mustMarshalSubscription(sub))
	if err != nil || tag.RowsAffected() != 1 {
		writeError(w, 500, "Não foi possível salvar a inscrição")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "Não foi possível salvar a inscrição")
		return
	}
	writeJSON(w, 200, map[string]bool{"enabled": true})
}
func mustMarshalSubscription(sub pushSubscription) []byte { b, _ := json.Marshal(sub); return b }

func pushRelayError(w http.ResponseWriter, status int) {
	if status == 429 {
		writeError(w, 429, "Limite de inscrições ou de envios atingido; tente novamente mais tarde")
		return
	}
	writeError(w, 502, "Serviço de notificações indisponível")
}
func (s *Server) handlePushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Endpoint string `json:"endpoint"`
	}
	if !pushRead(w, r, &v) {
		return
	}
	tx, err := s.store.Pool().Begin(r.Context())
	if err != nil {
		writeError(w, 500, "Não foi possível cancelar a inscrição")
		return
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", v.Endpoint); err != nil {
		writeError(w, 500, "Não foi possível cancelar a inscrição")
		return
	}
	var token string
	var owner *string
	err = tx.QueryRow(r.Context(), "SELECT user_id,management_token FROM push_subscriptions WHERE endpoint=$1", v.Endpoint).Scan(&owner, &token)
	if errors.Is(err, pgx.ErrNoRows) {
		writeNoContent(w)
		return
	}
	if err == nil && (owner == nil || *owner != userIDOf(r.Context())) {
		writeError(w, 403, "Inscrição vinculada a outra conta")
		return
	}
	if err != nil {
		writeError(w, 500, "Não foi possível localizar a inscrição")
		return
	}
	_, err = tx.Exec(r.Context(), "UPDATE push_subscriptions SET active=false, operation_id=md5(random()::text || clock_timestamp()::text), updated_at=now() WHERE endpoint=$1 AND user_id=$2", v.Endpoint, userIDOf(r.Context()))
	if err != nil {
		writeError(w, 500, "Não foi possível cancelar a inscrição")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "Não foi possível confirmar o cancelamento")
		return
	}
	// The scheduler removes inactive subscriptions from the relay, even after a restart.
	writeNoContent(w)
}

// removeUserPushSubscriptions revokes remote subscriptions before an account is deleted.
func (s *Server) removeUserPushSubscriptions(ctx context.Context, userID string) error {
	_, err := s.store.Pool().Exec(ctx, "UPDATE push_subscriptions SET active=false, operation_id=md5(random()::text || clock_timestamp()::text), updated_at=now() WHERE user_id=$1", userID)
	return err
}

func (s *Server) handlePushSend(w http.ResponseWriter, r *http.Request) {
	if !s.pushEnabled() {
		writeError(w, 503, "Notificações indisponíveis")
		return
	}
	var m struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		URL   string `json:"url"`
	}
	if !pushRead(w, r, &m) {
		return
	}
	m.Title = strings.TrimSpace(m.Title)
	m.Body = strings.TrimSpace(m.Body)
	m.URL = strings.TrimSpace(m.URL)
	if m.Title == "" || m.Body == "" || utf8.RuneCountInString(m.Title) > 120 || utf8.RuneCountInString(m.Body) > 500 || len(m.URL) > 1000 {
		writeError(w, 400, "Preencha título e mensagem válidos")
		return
	}
	if m.URL != "" {
		u, e := url.Parse(m.URL)
		if e != nil || (u.IsAbs() && (u.Scheme != "https" || u.Host == "" || u.User != nil)) || (!u.IsAbs() && (!strings.HasPrefix(m.URL, "/") || strings.HasPrefix(m.URL, "//"))) {
			writeError(w, 400, "Link deve ser HTTPS ou um caminho do aplicativo")
			return
		}
	}
	rows, err := s.store.Pool().Query(r.Context(), "SELECT relay_id FROM push_subscriptions WHERE user_id IS NOT NULL AND active ORDER BY relay_id")
	if err != nil {
		writeError(w, 500, "Não foi possível listar dispositivos")
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) != nil {
			rows.Close()
			writeError(w, 500, "Não foi possível listar dispositivos")
			return
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		writeError(w, 500, "Não foi possível listar dispositivos")
		return
	}
	queued := 0
	if m.URL == "" {
		m.URL = "/"
	}
	payload := map[string]any{"title": m.Title, "body": m.Body, "url": m.URL, "icon": "/icon-192.png"}
	if encoded, _ := json.Marshal(payload); len(encoded) > 3000 {
		writeError(w, 400, "Mensagem e link excedem o limite de tamanho")
		return
	}
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
		status, e := s.relay(r.Context(), "POST", "/api/send", "", map[string]any{"payload": payload, "subscriptionIds": ids[i:end]}, &result)
		if e != nil {
			writeJSON(w, 502, map[string]any{"error": fmt.Sprintf("%d dispositivos enfileirados; resultado incerto para %d restantes. Confira antes de reenviar", queued, len(ids)-i), "queued": queued, "remaining": len(ids) - i, "relayStatus": status})
			return
		}
		queued += result.Data.Queued
	}
	writeJSON(w, 200, map[string]int{"queued": queued, "selected": len(ids)})
}
