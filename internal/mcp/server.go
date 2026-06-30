// Package mcp wires the qBittorrent client into MCP tool handlers and the
// stdio transport.
package mcp

import (
	"context"
	"log/slog"

	qbt "github.com/autobrr/go-qbittorrent"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wyvernzora/qbit-bridge/internal/savepath"
)

const (
	serverName = "qbit-bridge"
)

// New constructs the MCP server with all qBittorrent tools registered.
// resolver is consulted by every tool that takes a destination input
// so callers pick from a deploy-time-configured list of named paths
// instead of supplying arbitrary filesystem paths.
func New(client *qbt.Client, resolver *savepath.Resolver, logger *slog.Logger, version string) *mcpsdk.Server {
	s := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    serverName,
		Version: version,
	}, nil)
	Register(s, client, resolver, logger)
	return s
}

// RunStdio runs the server on the stdio transport. Blocks until ctx is done or
// the transport returns.
func RunStdio(ctx context.Context, s *mcpsdk.Server) error {
	return s.Run(ctx, &mcpsdk.StdioTransport{})
}
