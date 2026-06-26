package rest

import (
	"net/http"

	"github.com/wyvernzora/qbittorrent-mcp/internal/downloads"
)

func (api api) removeDownload(w http.ResponseWriter, r *http.Request, hash string) {
	out, terr := api.downloads.Remove(r.Context(), downloads.RemoveDownloadsInput{
		HashesOrFilter: downloads.HashesOrFilter{Hashes: []string{hash}},
	})
	if terr != nil {
		writeToolError(w, terr)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
