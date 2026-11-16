package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"play-music/internal/config"
	"strings"
	"testing"
)

func TestRelayUsesServerSecretAndRespectsResponse(t *testing.T) {
	h := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "server-secret" {
			t.Error("missing server authorization")
		}
		if r.URL.Path != "/api/send" {
			t.Error("wrong path")
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("bad JSON")
		}
		w.WriteHeader(202)
		w.Write([]byte(`{"data":{"queued":1}}`))
	}))
	defer h.Close()
	old := pushHTTP
	pushHTTP = h.Client()
	defer func() { pushHTTP = old }()
	s := &Server{cfg: &config.Config{RelayBaseURL: h.URL, RelayAppSecret: "server-secret"}}
	var v struct {
		Data struct {
			Queued int `json:"queued"`
		} `json:"data"`
	}
	status, err := s.relay(context.Background(), "POST", "/api/send", "", map[string]any{"payload": map[string]string{"title": "Aviso"}}, &v)
	if err != nil || status != 202 || v.Data.Queued != 1 {
		t.Fatalf("status=%d queued=%d err=%v", status, v.Data.Queued, err)
	}
}
func TestPushConfigurationDoesNotExposeSecret(t *testing.T) {
	s := &Server{cfg: &config.Config{PushProvider: "relay", RelayAppID: "app", RelayAppSecret: "secret-value", RelayVapidPublicKey: "public"}}
	w := httptest.NewRecorder()
	s.handlePushConfig(w, httptest.NewRequest("GET", "/", nil))
	if strings.Contains(w.Body.String(), "secret-value") || !strings.Contains(w.Body.String(), "public") {
		t.Fatal("invalid push configuration response")
	}
}
func TestPushSendRejectsUnsafeLinksBeforeDatabase(t *testing.T) {
	s := &Server{cfg: &config.Config{PushProvider: "relay", RelayAppID: "app", RelayAppSecret: "secret", RelayVapidPublicKey: "public"}}
	for _, link := range []string{"javascript:alert(1)", "//evil.test", "http://evil.test"} {
		w := httptest.NewRecorder()
		body := `{"title":"Aviso","body":"Mensagem","url":` + string(mustJSON(link)) + `}`
		s.handlePushSend(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("%q returned %d", link, w.Code)
		}
	}
}
func mustJSON(v string) []byte { b, _ := json.Marshal(v); return b }
