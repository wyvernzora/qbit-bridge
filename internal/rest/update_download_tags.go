package rest

import (
	"net/http"

	"github.com/wyvernzora/qbit-bridge/internal/downloads"
)

type updateDownloadTagsRequest struct {
	Add    []string `json:"add,omitempty"`
	Remove []string `json:"remove,omitempty"`
}

func (api api) updateDownloadTags(w http.ResponseWriter, r *http.Request, hash string) {
	var body updateDownloadTagsRequest
	if terr := decodeJSON(r, &body); terr != nil {
		writeToolError(w, terr)
		return
	}
	out, terr := api.downloads.UpdateTags(r.Context(), downloads.UpdateDownloadTagsInput{
		Hashes: []string{hash},
		Add:    body.Add,
		Remove: body.Remove,
	})
	if terr != nil {
		writeToolError(w, terr)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
