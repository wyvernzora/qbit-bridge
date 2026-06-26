package rest

import (
	"net/http"

	"github.com/wyvernzora/qbittorrent-mcp/internal/downloads"
)

func (api api) addDownload(w http.ResponseWriter, r *http.Request) {
	var in downloads.AddDownloadInput
	if terr := decodeJSON(r, &in); terr != nil {
		writeToolError(w, terr)
		return
	}
	out, terr := api.downloads.Add(r.Context(), in)
	if terr != nil {
		writeToolError(w, terr)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
