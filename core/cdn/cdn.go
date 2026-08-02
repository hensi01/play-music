// Package cdn generates signed Bunny CDN URLs for serving audio files from an
// S3/MinIO origin through the Bunny edge network.
//
// The Pull Zone is configured with the MinIO bucket (or a folder inside it) as
// its S3 origin. Navidrome signs the object URL with Bunny's token
// authentication so only authenticated users can access the files.
//
// Two signing schemes are supported (conf.Server.CDN.AdvancedAuth):
//   - Advanced (default): HMAC-SHA256 tokens ("HS256-..."), recommended by Bunny.
//   - Basic: legacy MD5 tokens.
package cdn

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/hensi01/play-music/conf"
)

// Enabled reports whether the Bunny CDN is configured and ready to serve
// streams. Requires the base URL and the token authentication key.
func Enabled() bool {
	return conf.Server.CDN.Enabled &&
		conf.Server.CDN.BaseURL != "" &&
		conf.Server.CDN.TokenAuthKey != ""
}

// StreamURL builds a signed Bunny CDN URL for the given library-relative media
// path (e.g. "Artist/Album/track.mp3"). The CDN path is
// "<CDN.PathPrefix>/<relPath>", so the Pull Zone origin must serve that path
// from the S3 bucket. Returns ok=false when the CDN is not configured.
//
// Per Bunny's token authentication reference implementations (the official
// BunnyWay/BunnyCDN.TokenAuthentication libraries), the signature is computed
// over the DECODED path (e.g. "/Test Artist/track.mp3"), while the request URL
// carries the percent-encoded path. Encoding the path before signing produces
// a 403 from the edge.
func StreamURL(relPath string) (string, bool) {
	if !Enabled() {
		return "", false
	}
	base := strings.TrimRight(conf.Server.CDN.BaseURL, "/")
	cdnPath := path.Join(conf.Server.CDN.PathPrefix, relPath)
	cdnPath = "/" + strings.TrimLeft(cdnPath, "/")
	signPath := path.Clean(cdnPath)
	// The URL-encoded path, used only on the request line.
	encodedPath := escapePath(cdnPath)
	expires := strconv.FormatInt(time.Now().Add(conf.Server.CDN.TokenTTL).Unix(), 10)

	if conf.Server.CDN.AdvancedAuth {
		return signAdvanced(base, signPath, encodedPath, expires), true
	}
	return signBasic(base, signPath, encodedPath, expires), true
}

// escapePath URL-encodes each path segment while preserving the "/" separators.
func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		escaped := url.PathEscape(part)
		// url.PathEscape leaves some characters valid in a path (e.g. "&")
		// that are ambiguous inside a URL string. Escape them explicitly so the
		// URL and the signed path always match byte-for-byte.
		escaped = strings.ReplaceAll(escaped, "&", "%26")
		parts[i] = escaped
	}
	return strings.Join(parts, "/")
}

// signBasic implements Bunny's Basic Token Authentication:
//
//	token = Base64URL(MD5(security_key + path + expires))
func signBasic(base, signPath, encodedPath, expires string) string {
	sum := md5.Sum([]byte(conf.Server.CDN.TokenAuthKey + signPath + expires))
	token := base64.RawURLEncoding.EncodeToString(sum[:])
	return fmt.Sprintf("%s%s?token=%s&expires=%s", base, encodedPath, token, expires)
}

// signAdvanced implements Bunny's Advanced Token Authentication (HMAC-SHA256).
// It sets token_ignore_params=true so clients (e.g. the web player) can append
// cache-busting query parameters without invalidating the token.
//
//	token = "HS256-" + Base64URL(HMAC-SHA256(security_key, path + expires + signing_data))
func signAdvanced(base, signPath, encodedPath, expires string) string {
	signingData := "token_ignore_params=true"
	message := signPath + expires + signingData
	mac := hmac.New(sha256.New, []byte(conf.Server.CDN.TokenAuthKey))
	_, _ = mac.Write([]byte(message))
	token := "HS256-" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s%s?token=%s&expires=%s&%s", base, encodedPath, token, expires, signingData)
}
