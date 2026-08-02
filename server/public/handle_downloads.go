package public

import (
	"cmp"
	"fmt"
	"net/http"
	"strings"

	"github.com/hensi01/play-music/model"
	"github.com/hensi01/play-music/utils/req"
	"github.com/hensi01/play-music/utils/str"
)

func (pub *Router) handleDownloads(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := req.Params(r).String(":id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Load the share before streaming: once ZipShare writes its first byte the
	// status is locked at 200, so errors could no longer be reported.
	s, err := pub.share.Load(ctx, id)
	if err != nil {
		checkShareError(ctx, w, err, id)
		return
	}
	if !s.Downloadable {
		checkShareError(ctx, w, model.ErrNotAuthorized, id)
		return
	}

	name := str.SanitizeFilename(cmp.Or(s.Description, s.ID))
	name = strings.ReplaceAll(name, ",", "_")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name+".zip"))
	w.Header().Set("Content-Type", "application/zip")

	err = pub.archiver.ZipShare(ctx, s, w)
	checkShareError(ctx, w, err, id)
}
