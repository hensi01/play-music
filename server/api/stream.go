package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/hensi01/play-music/core/cdn"
	"github.com/hensi01/play-music/log"
	"github.com/hensi01/play-music/model"
)

// stream serves the audio for a media file. When the source is on S3/MinIO and
// the Bunny CDN is configured, a raw (untranscoded) request is answered with a
// 307 redirect to a signed CDN URL. Otherwise the bytes are proxied, possibly
// through ffmpeg when a transcode format was requested.
func (api *Router) stream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	format := r.URL.Query().Get("format")
	maxBitRate, _ := strconv.Atoi(r.URL.Query().Get("maxBitRate"))
	timeOffset, _ := strconv.Atoi(r.URL.Query().Get("timeOffset"))

	mf, err := api.ds.MediaFile(ctx).Get(id)
	if err != nil {
		api.handleError(w, r, err)
		return
	}

	streamReq := api.transcodeDecider.ResolveRequest(ctx, mf, format, maxBitRate, timeOffset)

	// Bunny CDN mode: when the track can be served directly and its library
	// lives on S3/MinIO, redirect the client to a signed CDN URL.
	if cdnRedirectEnabled(mf) && timeOffset == 0 && streamReq.Format == "raw" {
		if u, ok := cdn.StreamURL(mf.Path); ok {
			log.Debug(ctx, "CDN: redirecting stream to Bunny CDN", "id", id, "url", u)
			http.Redirect(w, r, u, http.StatusTemporaryRedirect)
			return
		}
	}

	stream, err := api.streamer.NewStream(ctx, mf, streamReq)
	if err != nil {
		api.handleError(w, r, err)
		return
	}
	defer func() {
		if err := stream.Close(); err != nil {
			log.Error(ctx, "Error closing stream", "id", id, "file", stream.Name(), err)
		}
	}()

	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Content-Duration", strconv.FormatFloat(float64(stream.Duration()), 'G', -1, 32))

	if _, err := stream.Serve(ctx, w, r); err != nil {
		log.Error(ctx, "Error serving stream", "id", id, "file", stream.Name(), err)
	}
}

// cdnRedirectEnabled reports whether Bunny CDN redirects are active for a media
// file. Only S3/MinIO-backed libraries qualify, because mf.Path must map
// directly to an object key served by the Pull Zone origin.
func cdnRedirectEnabled(mf *model.MediaFile) bool {
	if !cdn.Enabled() {
		return false
	}
	return strings.HasPrefix(mf.LibraryPath, "s3://")
}
