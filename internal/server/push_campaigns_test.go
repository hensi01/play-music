package server

import (
	"testing"
)

func TestPushMessageValidation(t *testing.T) {
	cases := []struct {
		name string
		msg  pushMessage
		ok   bool
	}{
		{"default destination", pushMessage{Title: " Aviso ", Body: " Mensagem "}, true},
		{"https destination", pushMessage{Title: "Aviso", Body: "Mensagem", URL: "https://example.test/oferta"}, true},
		{"script destination", pushMessage{Title: "Aviso", Body: "Mensagem", URL: "javascript:alert(1)"}, false},
		{"external protocol relative", pushMessage{Title: "Aviso", Body: "Mensagem", URL: "//example.test"}, false},
		{"blank title", pushMessage{Title: " ", Body: "Mensagem"}, false},
		{"oversized body", pushMessage{Title: "Aviso", Body: repeatRunes('a', 501)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := tc.msg
			err := msg.validate()
			if (err == nil) != tc.ok {
				t.Fatalf("ok=%v err=%v", tc.ok, err)
			}
			if tc.ok && msg.URL == "" {
				t.Fatal("missing default destination")
			}
		})
	}
}

// The relay rejects payloads above 3000 bytes, so the bound is ours to enforce.
func TestPushMessagePayloadSizeLimit(t *testing.T) {
	msg := pushMessage{
		Title: repeatRunes('🔥', 120),
		Body:  repeatRunes('🔥', 500),
		URL:   "/" + repeatRunes('x', 999),
	}
	if err := msg.validate(); err != nil {
		t.Fatalf("message should validate: %v", err)
	}
	if _, err := msg.payload(); err == nil {
		t.Fatal("oversized payload was accepted")
	}
}

func repeatRunes(r rune, n int) string {
	out := make([]rune, n)
	for i := range out {
		out[i] = r
	}
	return string(out)
}
