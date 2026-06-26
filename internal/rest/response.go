package rest

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/wyvernzora/qbit-bridge/internal/downloads"
)

func decodeJSON(r *http.Request, dst any) *downloads.ToolError {
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return &downloads.ToolError{Code: downloads.CodeInvalidArgument, Message: "invalid json body: " + err.Error(), Retriable: false}
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return &downloads.ToolError{Code: downloads.CodeInvalidArgument, Message: "invalid json body: multiple json values", Retriable: false}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeToolError(w http.ResponseWriter, terr *downloads.ToolError) {
	writeJSON(w, statusForToolError(terr), terr)
}

func statusForToolError(terr *downloads.ToolError) int {
	switch terr.Code {
	case downloads.CodeInvalidArgument:
		return http.StatusBadRequest
	case downloads.CodeUpstreamNotFound:
		return http.StatusNotFound
	case downloads.CodeUpstreamUnavailable, downloads.CodeUpstreamForbidden:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

func methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	writeJSON(w, http.StatusMethodNotAllowed, &downloads.ToolError{
		Code:      downloads.CodeInvalidArgument,
		Message:   "method not allowed",
		Retriable: false,
	})
}
