package rest

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/wyvernzora/qbit-bridge/internal/downloads"
)

func (api api) listDownloads(w http.ResponseWriter, r *http.Request) {
	in, terr := searchInputFromQuery(r)
	if terr != nil {
		writeToolError(w, terr)
		return
	}
	out, terr := api.downloads.Search(r.Context(), in)
	if terr != nil {
		writeToolError(w, terr)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func searchInputFromQuery(r *http.Request) (downloads.SearchDownloadsInput, *downloads.ToolError) {
	q := r.URL.Query()
	in := downloads.SearchDownloadsInput{
		Tags:          q["tags"],
		Hashes:        q["hashes"],
		Sort:          q.Get("sort"),
		IncludeFields: q["include_fields"],
	}
	for _, s := range q["states"] {
		in.States = append(in.States, downloads.NormalizedState(s))
	}
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return downloads.SearchDownloadsInput{}, &downloads.ToolError{Code: downloads.CodeInvalidArgument, Message: fmt.Sprintf("limit must be an integer: %v", err), Retriable: false}
		}
		in.Limit = n
	}
	if raw := q.Get("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return downloads.SearchDownloadsInput{}, &downloads.ToolError{Code: downloads.CodeInvalidArgument, Message: fmt.Sprintf("offset must be an integer: %v", err), Retriable: false}
		}
		in.Offset = n
	}
	return in, nil
}
