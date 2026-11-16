package server

import "testing"

func TestValidateAds(t *testing.T) {
	valid := adConfig{Frequency: 2, Ads: []adEntry{{Name: "Teste", EmbedURL: "https://ads.nuvexaia.online/embed/7896f085-9598-4fc0-9013-6f3dcc809535", Duration: 30, Active: true}}}
	if !validateAds(&valid) {
		t.Fatal("valid ad rejected")
	}
	tests := []struct {
		name, url           string
		duration, frequency int
	}{
		{"wrong host", "https://ads.nuvexaia.online.evil.example/embed/a", 30, 1},
		{"http", "http://ads.nuvexaia.online/embed/a", 30, 1},
		{"path", "https://ads.nuvexaia.online/embed/a/b", 30, 1},
		{"query", "https://ads.nuvexaia.online/embed/a?x=1", 30, 1},
		{"duration", "https://ads.nuvexaia.online/embed/a", 0, 1},
		{"frequency", "https://ads.nuvexaia.online/embed/a", 30, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := adConfig{Frequency: tt.frequency, Ads: []adEntry{{Name: "A", EmbedURL: tt.url, Duration: tt.duration}}}
			if validateAds(&c) {
				t.Fatal("invalid ad accepted")
			}
		})
	}
	valid.Ads = append(valid.Ads, valid.Ads[0])
	if validateAds(&valid) {
		t.Fatal("duplicate ad accepted")
	}
}
