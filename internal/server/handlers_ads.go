package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// adHost is the only origin an ad embed may come from: the partner serves the
// embeds, so this is an external contract, not our routing.
const adHost = "ads.nuvexaia.online"

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
		if err != nil || u.Scheme != "https" || u.Host != adHost || u.User != nil ||
			u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, "/embed/") {
			return false
		}
		id := strings.TrimPrefix(u.Path, "/embed/")
		if !adID.MatchString(id) || seen[id] || a.Name == "" || len(a.Name) > 120 ||
			a.Duration < 1 || a.Duration > 3600 {
			return false
		}
		seen[id] = true
		a.EmbedURL = "https://" + adHost + "/embed/" + id
	}
	if c.Ads == nil {
		c.Ads = []adEntry{}
	}
	return true
}

// adsSettingKey is the settings row the ad configuration lives in. The reader
// and the writer must name the same row; nothing checks that they do.
const adsSettingKey = "ads_config"

func (s *Server) readAds(r *http.Request) (adConfig, error) {
	c := adConfig{Frequency: 1, Ads: []adEntry{}}
	raw, ok, err := s.store.GetSetting(r.Context(), adsSettingKey)
	if err != nil || !ok {
		return c, err
	}
	err = json.Unmarshal([]byte(raw), &c)
	return c, err
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
	if !decodeJSON(w, r, &c) {
		return
	}
	if !validateAds(&c) {
		writeError(w, http.StatusBadRequest, "Configuração de anúncios inválida")
		return
	}
	b, _ := json.Marshal(c)
	if err := s.store.SetSetting(r.Context(), adsSettingKey, string(b)); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}
