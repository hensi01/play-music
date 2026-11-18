package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"play-music/internal/config"
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

func TestPushEnrollmentRoutesRemainAvailableBeforeActivation(t *testing.T) {
	s := &Server{}
	for _, pattern := range []string{
		"GET /api/me",
		"GET /api/push/config",
		"POST /api/push/status",
		"POST /api/push/subscriptions",
		"DELETE /api/push/subscriptions",
	} {
		req := httptest.NewRequest("GET", pattern[strings.IndexByte(pattern, ' ')+1:], nil)
		req.Pattern = pattern
		w := httptest.NewRecorder()
		if !s.pushAccessAllowed(w, req, "user") {
			t.Fatalf("enrollment route unexpectedly blocked: %s", pattern)
		}
	}
}

func TestChatGPTModeCanUsePlayerWithoutSubscription(t *testing.T) {
	s := &Server{}
	cases := []struct {
		name   string
		path   string
		header bool
	}{
		{name: "API requests", path: "/api/songs", header: true},
		{name: "audio element", path: "/api/stream/song-1?pushUnavailable=1"},
		{name: "artwork image", path: "/api/artwork/song-1?pushUnavailable=1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Pattern = "GET " + tc.path
			if tc.header {
				req.Header.Set(pushUnavailableHeader, "1")
			}
			w := httptest.NewRecorder()
			if !s.pushAccessAllowed(w, req, "user") || w.Code != http.StatusOK {
				t.Fatalf("unsupported client was blocked: status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestRelayValidationErrorIsCategorizedWithoutLeakingDetails(t *testing.T) {
	for _, tc := range []struct{ name, response, want string }{
		{"unsupported", `{"code":"VALIDATION_ERROR","error":"The push endpoint is not a supported browser Web Push provider"}`, "unsupported_provider"},
		{"other", `{"code":"VALIDATION_ERROR","error":"private subscription data"}`, "invalid_subscription"},
		{"malformed", `private subscription data`, "invalid_subscription"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(tc.response))
			}))
			defer h.Close()
			old := pushHTTP
			pushHTTP = h.Client()
			defer func() { pushHTTP = old }()
			s := &Server{cfg: &config.Config{RelayBaseURL: h.URL}, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
			status, err := s.relay(context.Background(), "POST", "/test", "", nil, nil)
			var relayErr *relayRequestError
			if status != 422 || !errors.As(err, &relayErr) || relayErr.category != tc.want || strings.Contains(err.Error(), "private") {
				t.Fatalf("status=%d err=%v", status, err)
			}
			w := httptest.NewRecorder()
			s.pushRelayError(w, "enrollment_ticket", status, relayErr)
			if w.Code != 422 || strings.Contains(w.Body.String(), "private") {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}
