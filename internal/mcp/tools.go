package mcp

import (
	"context"
	"log/slog"
	"time"

	qbt "github.com/autobrr/go-qbittorrent"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wyvernzora/qbit-bridge/internal/savepath"
)

// internalHandler is the shape every tool implements internally. The caller
// (wrap) is responsible for translating *ToolError into the SDK's error
// result and recording the err code for logs without round-tripping JSON.
type internalHandler[I, O any] func(ctx context.Context, in I) (O, *ToolError)

// Register adds all qBittorrent tools to the given server. The tool surface
// is split by domain — see internal/mcp/tools_downloads.go,
// internal/mcp/tools_tags.go, and internal/mcp/tools_destinations.go
// for the per-domain registrations.
// docs/tools.md is the design spec for the whole surface.
//
// resolver is the deploy-time destination alias map; tool handlers that
// accept a `destination` input translate the alias name into the upstream
// save_path via resolver.Resolve.
func Register(s *mcpsdk.Server, client *qbt.Client, resolver *savepath.Resolver, logger *slog.Logger) {
	registerDownloads(s, client, resolver, logger)
	registerTags(s, client, resolver, logger)
	registerDestinations(s, client, resolver, logger)
}

// wrap adapts an internalHandler into the SDK signature. It records logging
// at info level (without leaking the raw input) and produces a structured
// CallToolResult on error.
func wrap[I, O any](name string, logger *slog.Logger, h internalHandler[I, O]) mcpsdk.ToolHandlerFor[I, O] {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, in I) (*mcpsdk.CallToolResult, O, error) {
		start := time.Now()
		logger.Debug("tool call start", "tool", name, "input", in)
		out, terr := h(ctx, in)
		dur := time.Since(start)
		errCode := ""
		if terr != nil {
			errCode = string(terr.Code)
		}
		logger.Info("tool call",
			"tool", name,
			"duration_ms", dur.Milliseconds(),
			"err_code", errCode,
		)
		if terr != nil {
			return errorResult(terr), out, nil
		}
		return nil, out, nil
	}
}

// errorResult builds a CallToolResult carrying the structured ToolError JSON
// so MCP clients can branch on `code` programmatically.
func errorResult(te *ToolError) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		IsError: true,
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: te.JSON()},
		},
	}
}

// readOnlyAnnotations is the ToolAnnotations preset used by every
// read-only tool (qbit_search_*, qbit_list_*). qBittorrent is the
// operator's own instance — closed-world per the MCP spec's example
// framing (your-DB rather than open-internet). Reads are idempotent and
// never mutate.
//
//nolint:unused // referenced by the per-domain registrars in tools_*.go
func readOnlyAnnotations() *mcpsdk.ToolAnnotations {
	no := false
	return &mcpsdk.ToolAnnotations{
		ReadOnlyHint:    true,
		DestructiveHint: &no,
		IdempotentHint:  true,
		OpenWorldHint:   &no,
	}
}

// mutatingAnnotations is the preset for tools that mutate qBittorrent
// state. DestructiveHint is true only on the actually-destructive ops;
// the rest are non-destructive mutations. OpenWorldHint is false for the
// same reason as the read-only preset — qBittorrent is the operator's own
// instance.
//
//nolint:unused // referenced by the per-domain registrars in tools_*.go
func mutatingAnnotations(destructive bool) *mcpsdk.ToolAnnotations {
	no := false
	d := destructive
	return &mcpsdk.ToolAnnotations{
		ReadOnlyHint:    false,
		DestructiveHint: &d,
		IdempotentHint:  false,
		OpenWorldHint:   &no,
	}
}
