package mcp

import "encoding/json"

// ErrCode is a stable string identifier the agent can branch on.
type ErrCode string

const (
	CodeInvalidArgument     ErrCode = "invalid_argument"
	CodeUpstreamUnavailable ErrCode = "upstream_unavailable"
	// CodeUpstreamForbidden signals the loopback-auth-bypass assumption was
	// wrong. qBittorrent returned 401/403 (or autobrr's ErrBadCredentials /
	// ErrIPBanned bubbled up): the operator must enable "Bypass authentication
	// for clients on localhost" in the WebUI settings.
	CodeUpstreamForbidden ErrCode = "upstream_forbidden"
	CodeUpstreamNotFound  ErrCode = "upstream_not_found"
	CodeInternal          ErrCode = "internal"
)

// ToolError is the structured payload returned to MCP clients on tool failure.
// It implements error so it can flow through Go error returns.
type ToolError struct {
	Code      ErrCode `json:"code"`
	Message   string  `json:"message"`
	Retriable bool    `json:"retriable"`
}

func (e *ToolError) Error() string { return string(e.Code) + ": " + e.Message }

// JSON renders the ToolError as a single-line JSON object suitable for
// embedding in a TextContent body.
func (e *ToolError) JSON() string {
	b, err := json.Marshal(e)
	if err != nil {
		return `{"code":"internal","message":"failed to marshal error","retriable":false}`
	}
	return string(b)
}
