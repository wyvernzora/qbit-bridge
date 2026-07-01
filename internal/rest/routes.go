package rest

import (
	"net/http"
	"strings"

	"github.com/wyvernzora/qbit-bridge/internal/downloads"
)

const (
	// DownloadsPath is the collection route for download list/create operations.
	DownloadsPath       = "/api/v1/downloads"
	downloadsPathSlash  = "/api/v1/downloads/"
	tagsPathSuffix      = "/tags"
	errDownloadNotFound = "download not found"
)

type api struct {
	downloads *downloads.Service
}

// Register mounts REST download routes on mux.
func Register(mux *http.ServeMux, service *downloads.Service) {
	api := api{downloads: service}
	mux.HandleFunc(DownloadsPath, api.downloadsRoute)
	mux.HandleFunc(downloadsPathSlash, api.downloadRoute)
}

func (api api) downloadsRoute(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		api.listDownloads(w, r)
	case http.MethodPost:
		api.addDownload(w, r)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (api api) downloadRoute(w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimPrefix(r.URL.Path, downloadsPathSlash)
	if strings.HasSuffix(hash, tagsPathSuffix) {
		hash = strings.TrimSuffix(hash, tagsPathSuffix)
		if hash == "" || strings.Contains(hash, "/") {
			writeToolError(w, &downloads.ToolError{Code: downloads.CodeInvalidArgument, Message: "hash is required", Retriable: false})
			return
		}
		if r.Method != http.MethodPut {
			methodNotAllowed(w, http.MethodPut)
			return
		}
		api.updateDownloadTags(w, r, hash)
		return
	}

	if hash == "" || strings.Contains(hash, "/") {
		writeToolError(w, &downloads.ToolError{Code: downloads.CodeInvalidArgument, Message: "hash is required", Retriable: false})
		return
	}

	switch r.Method {
	case http.MethodGet:
		api.getDownload(w, r, hash)
	case http.MethodDelete:
		api.removeDownload(w, r, hash)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodDelete)
	}
}
