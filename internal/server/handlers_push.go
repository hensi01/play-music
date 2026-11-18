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

	"play-music/internal/model"
	"play-music/internal/store"
)

var pushHTTP = &http.Client{
	Timeout:       12 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

const pushUnavailableHeader = "X-Mi-Ritmo-Push-Unavailable"

func (s *Server) pushEnabled() bool {
	c := s.cfg
	return c.PushProvider == "relay" && c.RelayAppID != "" && c.RelayAppSecret != "" && c.RelayVapidPublicKey != ""
}

// relay calls the push relay. auth is the management token for authenticated
// calls; empty means the app secret authenticates the request.
func (s *Server) relay(ctx context.Context, method, path, auth string, body, result any) (int, error) {
	base := s.cfg.RelayBaseURL
	if base == "" {
		base = "https://vapid.party"
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return 0, errors.New("invalid relay URL")
	}
	b, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(base, "/")+path, bytes.NewReader(b))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	} else {
		req.Header.Set(apiKeyHeader, s.cfg.RelayAppSecret)
	}
	resp, err := pushHTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, errors.New("relay rejected request")
	}
	if result != nil {
		err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(result)
	}
	return resp.StatusCode, err
}

func (s *Server) relayPath(suffix string) string {
	return "/api/apps/" + url.PathEscape(s.cfg.RelayAppID) + suffix
}

type relayTicket struct {
	Data struct {
		Token string `json:"token"`
	} `json:"data"`
}

type relaySubscription struct {
	Data struct {
		ID         string `json:"subscriptionId"`
		Management struct {
			Token string `json:"token"`
		} `json:"management"`
	} `json:"data"`
}

type relaySendResult struct {
	Data struct {
		Queued int `json:"queued"`
	} `json:"data"`
}

// pushMessage is one notification. validate() is the single place the limits
// are enforced, for campaigns and immediate sends alike.
type pushMessage struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
}

func (m *pushMessage) validate() error {
	m.Title = strings.TrimSpace(m.Title)
	m.Body = strings.TrimSpace(m.Body)
	m.URL = strings.TrimSpace(m.URL)
	if m.Title == "" || m.Body == "" ||
		utf8.RuneCountInString(m.Title) > 120 || utf8.RuneCountInString(m.Body) > 500 || len(m.URL) > 1000 {
		return errors.New("Preencha título e mensagem válidos")
	}
	if m.URL == "" {
		m.URL = "/"
	}
	u, err := url.Parse(m.URL)
	if err != nil ||
		(u.IsAbs() && (u.Scheme != "https" || u.Host == "" || u.User != nil)) ||
		(!u.IsAbs() && (!strings.HasPrefix(m.URL, "/") || strings.HasPrefix(m.URL, "//"))) {
		return errors.New("Link deve ser HTTPS ou um caminho do aplicativo")
	}
	return nil
}

// payload is the object the relay hands to the service worker. The relay
// rejects payloads above 3000 bytes, so the bound is checked before sending.
// The service worker supplies its own icon and badge, so the payload carries
// only the fields it reads.
func (m pushMessage) payload() (map[string]any, error) {
	p := map[string]any{"title": m.Title, "body": m.Body, "url": m.URL}
	encoded, _ := json.Marshal(p)
	if len(encoded) > 3000 {
		return nil, errors.New("Mensagem e link excedem o limite de tamanho")
	}
	return p, nil
}

// relayFailure reports how far a batched send got before the relay failed, so
// the caller can tell "nothing sent" from "partially sent".
type relayFailure struct {
	StatusCode int
}

func (e *relayFailure) Error() string {
	return fmt.Sprintf("relay send failed with status %d", e.StatusCode)
}

// pushGateExempt reports whether a matched route may be reached before the user
// has an active push subscription. Enrolling and reading the account are what
// make activation possible, so they cannot require it; every other route in the
// table is gated. The pattern comes from the mux, so a new /api/push/ route is
// exempt without touching this middleware.
func pushGateExempt(pattern string) bool {
	path := pattern
	if i := strings.IndexByte(pattern, ' '); i >= 0 {
		path = pattern[i+1:]
	}
	return path == "/api/me" || strings.HasPrefix(path, "/api/push/")
}

func (s *Server) pushAccessAllowed(w http.ResponseWriter, r *http.Request, userID string) bool {
	if pushGateExempt(r.Pattern) {
		return true
	}
	// The ChatGPT link mode bypasses the push prerequisite; authentication is
	// still enforced by the surrounding middleware.
	if r.Header.Get(pushUnavailableHeader) == "1" ||
		(r.URL.Query().Get("pushUnavailable") == "1" &&
			(strings.HasPrefix(r.URL.Path, "/api/stream/") || strings.HasPrefix(r.URL.Path, "/api/artwork/"))) {
		return true
	}
	active, err := s.store.PushAccessActive(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "Não foi possível verificar notificações")
		return false
	}
	if !active {
		writeError(w, http.StatusPreconditionRequired, "Ative as notificações para usar o aplicativo")
		return false
	}
	return true
}

func (s *Server) handlePushStatus(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Endpoint string `json:"endpoint"`
	}
	if !decodeJSON(w, r, &v) {
		return
	}
	active, err := s.store.PushEndpointActive(r.Context(), v.Endpoint, userIDOf(r.Context()))
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "Não foi possível verificar notificações")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]bool{"active": active})
}

func (s *Server) handlePushConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":   s.pushEnabled(),
		"publicKey": s.cfg.RelayVapidPublicKey,
	})
}

type pushSubscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func (sub pushSubscription) marshal() []byte {
	b, _ := json.Marshal(sub)
	return b
}

func (s *Server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	if !s.pushEnabled() {
		writeError(w, http.StatusServiceUnavailable, "Notificações indisponíveis")
		return
	}
	var sub pushSubscription
	if !decodeJSON(w, r, &sub) {
		return
	}
	if len(sub.Endpoint) > 4096 || !strings.HasPrefix(sub.Endpoint, "https://") ||
		sub.Keys.P256dh == "" || sub.Keys.Auth == "" {
		writeError(w, http.StatusBadRequest, "Inscrição inválida")
		return
	}
	ctx := r.Context()
	uid := userIDOf(ctx)
	operationID := store.NewID()

	// An inactive row is persisted before the relay is contacted so a lost
	// response can still be recovered by the cleanup loop.
	if err := s.store.ReservePushEnrollment(ctx, sub.Endpoint, uid, operationID, sub.marshal()); err != nil {
		handleStoreError(w, err)
		return
	}

	credentials, status, err := s.enroll(ctx, sub)
	if err != nil {
		s.pushRelayError(w, "subscription", status, err)
		return
	}
	if err := s.store.ActivatePushSubscription(ctx, sub.Endpoint, uid, operationID,
		credentials.id, credentials.token, sub.marshal()); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": true})
}

// enroll performs the relay's ticket-then-subscription handshake. The returned
// management token is what later lets us revoke or refresh the subscription.
func (s *Server) enroll(ctx context.Context, sub pushSubscription) (managementCredentials, int, error) {
	missing := errors.New("relay returned no credentials")
	var ticket relayTicket
	status, err := s.relay(ctx, http.MethodPost, s.relayPath("/enrollment-ticket"), "", sub, &ticket)
	if err != nil {
		return managementCredentials{}, status, err
	}
	if ticket.Data.Token == "" {
		return managementCredentials{}, status, missing
	}

	var created relaySubscription
	status, err = s.relay(ctx, http.MethodPost, s.relayPath("/subscriptions"), ticket.Data.Token, sub, &created)
	if err != nil {
		return managementCredentials{}, status, err
	}
	if created.Data.ID == "" || created.Data.Management.Token == "" {
		return managementCredentials{}, status, missing
	}
	return managementCredentials{id: created.Data.ID, token: created.Data.Management.Token}, status, nil
}

type managementCredentials struct {
	id    string
	token string
}

func (s *Server) pushRelayError(w http.ResponseWriter, stage string, status int, cause error) {
	s.log.Warn("push relay request failed", "stage", stage, "upstream_status", status, "err", cause)
	if status == http.StatusTooManyRequests {
		writeError(w, http.StatusTooManyRequests, "Limite de inscrições ou de envios atingido; tente novamente mais tarde")
		return
	}
	writeError(w, http.StatusBadGateway, "Serviço de notificações indisponível")
}

func (s *Server) handlePushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Endpoint string `json:"endpoint"`
	}
	if !decodeJSON(w, r, &v) {
		return
	}
	// The device stays locally inactive; the scheduler revokes it remotely,
	// even after a restart.
	if err := s.store.DeactivatePushSubscription(r.Context(), v.Endpoint, userIDOf(r.Context())); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleCreateCampaign(w http.ResponseWriter, r *http.Request) {
	if !s.pushEnabled() {
		writeError(w, http.StatusServiceUnavailable, "Notificações indisponíveis")
		return
	}
	var req struct {
		pushMessage
		ScheduledAt time.Time `json:"scheduledAt"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	msg := req.pushMessage
	if err := msg.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := msg.payload(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ScheduledAt.IsZero() {
		req.ScheduledAt = time.Now().UTC()
	}
	if req.ScheduledAt.Before(time.Now().Add(-time.Minute)) {
		writeError(w, http.StatusBadRequest, "O horário não pode estar no passado")
		return
	}
	campaign := model.PushCampaign{
		ID:          store.NewID(),
		Title:       msg.Title,
		Body:        msg.Body,
		URL:         msg.URL,
		ScheduledAt: req.ScheduledAt,
		Status:      store.CampaignPending,
	}
	if err := s.store.CreatePushCampaign(r.Context(), &campaign, userIDOf(r.Context())); err != nil {
		writeError(w, http.StatusInternalServerError, "Não foi possível agendar")
		return
	}
	writeJSON(w, http.StatusCreated, campaign)
}

func (s *Server) handleListCampaigns(w http.ResponseWriter, r *http.Request) {
	campaigns, err := s.store.ListPushCampaigns(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Não foi possível consultar notificações")
		return
	}
	writeJSON(w, http.StatusOK, campaigns)
}

func (s *Server) handleCancelCampaign(w http.ResponseWriter, r *http.Request) {
	if err := s.store.CancelPushCampaign(r.Context(), r.PathValue("id")); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}
