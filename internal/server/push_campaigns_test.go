package server

import (
	"strings"
	"testing"
)

func TestCampaignValidation(t *testing.T) {
	cases := []struct {
		name     string
		campaign pushCampaign
		valid    bool
	}{
		{"default destination", pushCampaign{Title: " Aviso ", Body: " Mensagem "}, true},
		{"https destination", pushCampaign{Title: "Aviso", Body: "Mensagem", URL: "https://example.test/oferta"}, true},
		{"script destination", pushCampaign{Title: "Aviso", Body: "Mensagem", URL: "javascript:alert(1)"}, false},
		{"external protocol relative", pushCampaign{Title: "Aviso", Body: "Mensagem", URL: "//example.test"}, false},
		{"oversized unicode", pushCampaign{Title: "Aviso", Body: strings.Repeat("🔥", 500), URL: "/" + strings.Repeat("x", 999)}, false},
		{"blank title", pushCampaign{Title: " ", Body: "Mensagem"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validCampaign(&tc.campaign)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
			if tc.name == "default destination" && tc.campaign.URL != "/" {
				t.Fatal("missing default destination")
			}
		})
	}
}
