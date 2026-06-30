package mcp

import (
	"context"
	"log/slog"

	qbt "github.com/autobrr/go-qbittorrent"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wyvernzora/qbit-bridge/internal/savepath"
)

// registerDestinations wires the 1 destination tool onto s.
//
// The tool reads from the deploy-time-configured savepath resolver only;
// no upstream qBittorrent call is involved. client is accepted to keep
// the registrar signature uniform with the download / tag registrars.
func registerDestinations(s *mcpsdk.Server, _ *qbt.Client, resolver *savepath.Resolver, logger *slog.Logger) {
	mcpsdk.AddTool(s,
		&mcpsdk.Tool{
			Name:        "qbit_list_destinations",
			Description: "List the deploy-time-configured save destinations. Each entry is an alias name (used as the `destination` value on qbit_add_download) paired with the absolute filesystem path it resolves to. Agents that observed a raw save_path on a Download output can reverse-look it up here to find the matching alias name. The list is fixed for the lifetime of the qbit-bridge process; restart with a different --save-paths to change it.",
			Annotations: readOnlyAnnotations(),
		},
		wrap("qbit_list_destinations", logger, listDestinationsHandler(resolver)),
	)
}

// --- qbit_list_destinations ---

// ListDestinationsInput has no fields because destinations are deploy-time configuration.
type ListDestinationsInput struct{}

// Destination is one configured save-path alias.
type Destination struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// ListDestinationsOutput reports every configured save-path alias.
type ListDestinationsOutput struct {
	Destinations []Destination `json:"destinations"`
}

func listDestinationsHandler(resolver *savepath.Resolver) internalHandler[ListDestinationsInput, ListDestinationsOutput] {
	return func(_ context.Context, _ ListDestinationsInput) (ListDestinationsOutput, *ToolError) {
		names := resolver.Names()
		out := make([]Destination, 0, len(names))
		for _, name := range names {
			path, err := resolver.Resolve(name)
			if err != nil {
				// Defensive: Resolve should not fail on a name returned by Names().
				return ListDestinationsOutput{Destinations: []Destination{}}, &ToolError{
					Code: CodeInternal, Message: err.Error(), Retriable: false,
				}
			}
			out = append(out, Destination{Name: name, Path: path})
		}
		return ListDestinationsOutput{Destinations: out}, nil
	}
}
