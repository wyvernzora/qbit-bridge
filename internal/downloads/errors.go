package downloads

import (
	"encoding/json"
	"errors"
	"strings"

	qbt "github.com/autobrr/go-qbittorrent"
)

// ErrCode is a stable string identifier transport adapters can branch on.
type ErrCode string

const (
	CodeInvalidArgument     ErrCode = "invalid_argument"
	CodeUpstreamUnavailable ErrCode = "upstream_unavailable"
	CodeUpstreamForbidden   ErrCode = "upstream_forbidden"
	CodeUpstreamNotFound    ErrCode = "upstream_not_found"
	CodeInternal            ErrCode = "internal"
)

// ToolError is the structured error payload shared by MCP and REST adapters.
type ToolError struct {
	Code      ErrCode `json:"code"`
	Message   string  `json:"message"`
	Retriable bool    `json:"retriable"`
}

func (e *ToolError) Error() string { return string(e.Code) + ": " + e.Message }

// JSON renders the ToolError as a single-line JSON object.
func (e *ToolError) JSON() string {
	b, err := json.Marshal(e)
	if err != nil {
		return `{"code":"internal","message":"failed to marshal error","retriable":false}`
	}
	return string(b)
}

func errorFromSDK(err error) *ToolError {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case errors.Is(err, qbt.ErrBadCredentials),
		errors.Is(err, qbt.ErrIPBanned),
		strings.Contains(msg, "status code: 401"),
		strings.Contains(msg, "status code: 403"),
		strings.Contains(msg, "qbit re-login"):
		return &ToolError{Code: CodeUpstreamForbidden, Message: msg, Retriable: false}
	case errors.Is(err, qbt.ErrTorrentNotFound):
		return &ToolError{Code: CodeUpstreamNotFound, Message: msg, Retriable: false}
	case errors.Is(err, qbt.ErrInvalidTorrentHash),
		errors.Is(err, qbt.ErrEmptyTorrentName),
		errors.Is(err, qbt.ErrInvalidPriority),
		errors.Is(err, qbt.ErrInvalidURL):
		return &ToolError{Code: CodeInvalidArgument, Message: msg, Retriable: false}
	default:
		return &ToolError{Code: CodeUpstreamUnavailable, Message: msg, Retriable: true}
	}
}
