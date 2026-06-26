package mcp

import (
	"context"
	"log/slog"

	qbt "github.com/autobrr/go-qbittorrent"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wyvernzora/qbittorrent-mcp/internal/downloads"
	"github.com/wyvernzora/qbittorrent-mcp/internal/savepath"
)

// registerDownloads wires the 3 download tools onto s:
// qbit_search_downloads, qbit_add_download, qbit_remove_downloads.
func registerDownloads(s *mcpsdk.Server, client *qbt.Client, resolver *savepath.Resolver, logger *slog.Logger) {
	service := downloads.New(client, resolver, logger)
	destHint := resolver.DescriptionHint()

	mcpsdk.AddTool(s,
		&mcpsdk.Tool{
			Name:        "qbit_search_downloads",
			Description: "Search downloads with filtering, sorting, and pagination. Default projection is lean (hash, name, state, progress, sizes, speeds, eta, ratio, tags, added_on). Use include_fields to opt into richer fields: save_path, content_path, magnet_uri, peer/seed counts, total bytes, seeding_time, private. The special value include_fields=[\"all\"] expands every field-level key (but not trackers/files). The trackers and files keys trigger one additional upstream call per result hash — response size scales with len(hashes) * per-torrent fan-out, so opt in deliberately. Default limit 50, max 200; paginate via offset.",
			Annotations: readOnlyAnnotations(),
		},
		wrap("qbit_search_downloads", logger, adaptDownload(service.Search)),
	)
	mcpsdk.AddTool(s,
		&mcpsdk.Tool{
			Name:        "qbit_add_download",
			Description: "Add a download by magnet URI (URLs and .torrent file uploads are not supported in v1). The hash is parsed from the magnet's xt=urn:btih: parameter and returned synchronously. Idempotent: re-adding a hash qBittorrent already knows about leaves the live download untouched and reports already_existed=true. The destination field selects a deploy-time-configured save destination by name; raw filesystem paths are not accepted. " + destHint,
			Annotations: mutatingAnnotations(false),
		},
		wrap("qbit_add_download", logger, adaptDownload(service.Add)),
	)
	mcpsdk.AddTool(s,
		&mcpsdk.Tool{
			Name:        "qbit_remove_downloads",
			Description: "Remove downloads from qBittorrent's tracking. Pass exactly one of hashes (explicit set, possibly empty) or filter (states/tags). An explicitly-empty hashes array is a no-op success (affected_count=0) — safe for defensive cleanup loops. Omitting BOTH hashes and filter is rejected (we refuse to guess between no-op and operate-on-every-download). Files on disk are not deleted — file lifecycle is an operator concern (cron, kura's trash, manual rm). This tool only forgets the download from qbit's perspective.",
			Annotations: mutatingAnnotations(true),
		},
		wrap("qbit_remove_downloads", logger, adaptDownload(service.Remove)),
	)
}

func adaptDownload[I, O any](h func(context.Context, I) (O, *downloads.ToolError)) internalHandler[I, O] {
	return func(ctx context.Context, in I) (O, *ToolError) {
		out, err := h(ctx, in)
		if err != nil {
			return out, &ToolError{
				Code:      ErrCode(err.Code),
				Message:   err.Message,
				Retriable: err.Retriable,
			}
		}
		return out, nil
	}
}
