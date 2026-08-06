package web

import "embed"

// Assets embeds the vanilla JS web UI (web/assets) so the backend can serve
// the app and the API from a single binary.
//
//go:embed assets
var Assets embed.FS
