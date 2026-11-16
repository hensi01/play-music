package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type adEntry struct {
	Name     string `json:"name"`
	EmbedURL string `json:"embedUrl"`
	Duration int    `json:"duration"`
	Active   bool   `json:"active"`
}
type adConfig struct {
	Frequency int       `json:"frequency"`
	Ads       []adEntry `json:"ads"`
}

var adID = regexp.MustCompile(`^[a-zA-Z0-9-]{1,80}$`)

func validateAds(c *adConfig) bool {
	if c.Frequency < 1 || c.Frequency > 100 || len(c.Ads) > 100 {
		return false
	}
	seen := map[string]bool{}
	for i := range c.Ads {
		a := &c.Ads[i]
		a.Name = strings.TrimSpace(a.Name)
		u, err := url.Parse(strings.TrimSpace(a.EmbedURL))
		if err != nil || u.Scheme != "https" || u.Host != "ads.nuvexaia.online" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, "/embed/") {
			return false
		}
		id := strings.TrimPrefix(u.Path, "/embed/")
		if !adID.MatchString(id) || seen[id] || a.Name == "" || len(a.Name) > 120 || a.Duration < 1 || a.Duration > 3600 {
			return false
		}
		seen[id] = true
		a.EmbedURL = "https://ads.nuvexaia.online/embed/" + id
	}
	if c.Ads == nil {
		c.Ads = []adEntry{}
	}
	return true
}

func (s *Server) readAds(r *http.Request) (adConfig, error) {
	c := adConfig{Frequency: 1, Ads: []adEntry{}}
	raw, ok, err := s.store.GetSetting(r.Context(), "ads_config")
	if err != nil || !ok {
		return c, err
	}
	if err = json.Unmarshal([]byte(raw), &c); err != nil {
		return c, err
	}
	return c, nil
}
func (s *Server) handleAdminAds(w http.ResponseWriter, r *http.Request) {
	c, err := s.readAds(r)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, c)
}
func (s *Server) handleAds(w http.ResponseWriter, r *http.Request) {
	c, err := s.readAds(r)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	active := []adEntry{}
	for _, a := range c.Ads {
		if a.Active {
			active = append(active, a)
		}
	}
	c.Ads = active
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, c)
}
func (s *Server) handleSaveAds(w http.ResponseWriter, r *http.Request) {
	var c adConfig
	if err := json.NewDecoder(io.LimitReader(r.Body, 65536)).Decode(&c); err != nil || !validateAds(&c) {
		writeError(w, http.StatusBadRequest, "Configuração de anúncios inválida")
		return
	}
	b, _ := json.Marshal(c)
	if err := s.store.SetSetting(r.Context(), "ads_config", string(b)); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}
