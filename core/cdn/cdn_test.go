package cdn

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/hensi01/play-music/conf"
)

func setup(t *testing.T) {
	t.Helper()
	conf.Server.CDN.Enabled = true
	conf.Server.CDN.BaseURL = "https://music.centralcursoss.com.br"
	conf.Server.CDN.TokenAuthKey = "test-security-key"
	conf.Server.CDN.TokenTTL = time.Hour
	conf.Server.CDN.PathPrefix = ""
}

func TestEnabled(t *testing.T) {
	setup(t)
	if !Enabled() {
		t.Fatal("expected CDN enabled")
	}
	conf.Server.CDN.Enabled = false
	if Enabled() {
		t.Fatal("expected CDN disabled")
	}
	conf.Server.CDN.Enabled = true
	conf.Server.CDN.TokenAuthKey = ""
	if Enabled() {
		t.Fatal("expected CDN disabled without token key")
	}
}

func TestSignBasicMatchesBunnyAlgorithm(t *testing.T) {
	setup(t)
	conf.Server.CDN.AdvancedAuth = false
	const relPath = "Artist/Album/track.mp3"

	u, ok := StreamURL(relPath)
	if !ok {
		t.Fatal("expected URL")
	}
	if !strings.HasPrefix(u, "https://music.centralcursoss.com.br/Artist/Album/track.mp3?") {
		t.Fatalf("unexpected URL prefix: %s", u)
	}

	// Recompute the expected token with Bunny's documented algorithm:
	// token = Base64URL(MD5(security_key + path + expires))
	params := queryParams(u)
	expires := params["expires"]
	if expires == "" {
		t.Fatalf("missing expires param in %s", u)
	}
	pathPart := "/Artist/Album/track.mp3"
	sum := md5.Sum([]byte("test-security-key" + pathPart + expires))
	expectedToken := base64.RawURLEncoding.EncodeToString(sum[:])
	if params["token"] != expectedToken {
		t.Fatalf("basic token mismatch: got %q want %q", params["token"], expectedToken)
	}
}

func TestSignAdvancedMatchesBunnyAlgorithm(t *testing.T) {
	setup(t)
	conf.Server.CDN.AdvancedAuth = true
	const relPath = "Artist/Album/track.mp3"

	u, ok := StreamURL(relPath)
	if !ok {
		t.Fatal("expected URL")
	}
	params := queryParams(u)
	expires := params["expires"]
	if expires == "" {
		t.Fatalf("missing expires param in %s", u)
	}
	if params["token_ignore_params"] != "true" {
		t.Fatalf("expected token_ignore_params=true, got %q", params["token_ignore_params"])
	}

	// Recompute the expected token:
	// token = "HS256-" + Base64URL(HMAC-SHA256(security_key, path + expires + signing_data))
	message := "/Artist/Album/track.mp3" + expires + "token_ignore_params=true"
	mac := hmac.New(sha256.New, []byte("test-security-key"))
	_, _ = mac.Write([]byte(message))
	expectedToken := "HS256-" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if params["token"] != expectedToken {
		t.Fatalf("advanced token mismatch: got %q want %q", params["token"], expectedToken)
	}
}

func TestStreamURLDisabled(t *testing.T) {
	conf.Server.CDN.Enabled = false
	if _, ok := StreamURL("x.mp3"); ok {
		t.Fatal("expected StreamURL to return ok=false when disabled")
	}
}

func TestStreamURLPathPrefix(t *testing.T) {
	setup(t)
	conf.Server.CDN.AdvancedAuth = false
	conf.Server.CDN.PathPrefix = "music"
	u, ok := StreamURL("Artist/Album/track.mp3")
	if !ok {
		t.Fatal("expected URL")
	}
	if !strings.HasPrefix(u, "https://music.centralcursoss.com.br/music/Artist/Album/track.mp3?") {
		t.Fatalf("expected path prefix in URL, got %s", u)
	}
}

func TestStreamURLEscapesSpecialChars(t *testing.T) {
	setup(t)
	conf.Server.CDN.AdvancedAuth = false
	u, ok := StreamURL("Artist/Album 1/01 Track & More.mp3")
	if !ok {
		t.Fatal("expected URL")
	}
	if !strings.Contains(u, "/Album%201/01%20Track%20%26%20More.mp3") {
		t.Fatalf("expected escaped path, got %s", u)
	}
}

// queryParams parses the query string of a URL.
func queryParams(u string) map[string]string {
	idx := strings.Index(u, "?")
	if idx < 0 {
		return map[string]string{}
	}
	res := map[string]string{}
	for _, kv := range strings.Split(u[idx+1:], "&") {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			res[parts[0]] = parts[1]
		}
	}
	return res
}
