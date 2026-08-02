package api

import (
	"bytes"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// getArtwork serves cover art / artist images. ids are plain entity ids
// (album/artist/song/playlist); the artwork service resolves the right image,
// falling back to a placeholder when none exists.
func (api *Router) getArtwork(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	size := queryInt(r, "size", 0)
	square := r.URL.Query().Get("square") == "true" || r.URL.Query().Get("square") == "1"

	reader, lastUpdate, err := api.artwork.GetOrPlaceholder(ctx, id, size, square)
	if err != nil {
		api.handleError(w, r, err)
		return
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		api.handleError(w, r, err)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=315360000")
	w.Header().Set("Content-Type", detectImageType(data))
	http.ServeContent(w, r, "", lastUpdate, bytes.NewReader(data))
}

func detectImageType(data []byte) string {
	// Go's DetectContentType does not recognize WebP.
	if len(data) > 12 && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	return http.DetectContentType(data)
}
