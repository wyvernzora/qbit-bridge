package rest

import (
	"net/http"

	"github.com/wyvernzora/qbit-bridge/internal/downloads"
)

func (api api) getDownload(w http.ResponseWriter, r *http.Request, hash string) {
	in, terr := searchInputFromQuery(r)
	if terr != nil {
		writeToolError(w, terr)
		return
	}
	in.Hashes = []string{hash}
	in.Limit = 1
	in.Offset = 0

	out, terr := api.downloads.Search(r.Context(), in)
	if terr != nil {
		writeToolError(w, terr)
		return
	}
	if len(out.Downloads) == 0 {
		writeToolError(w, &downloads.ToolError{Code: downloads.CodeUpstreamNotFound, Message: errDownloadNotFound, Retriable: false})
		return
	}
	writeJSON(w, http.StatusOK, out.Downloads[0])
}
