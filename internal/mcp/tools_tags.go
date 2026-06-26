package mcp

import (
	"context"
	"log/slog"

	qbt "github.com/autobrr/go-qbittorrent"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wyvernzora/qbit-bridge/internal/savepath"
)

// registerTags wires the 1 tag tool onto s. Tags auto-create when referenced
// via add_download.tags, so there is no create_tag tool.
//
// resolver is accepted to keep the registrar signature uniform with the
// download and subscription registrars; tag tools do not read it.
func registerTags(s *mcpsdk.Server, client *qbt.Client, _ *savepath.Resolver, logger *slog.Logger) {
	mcpsdk.AddTool(s,
		&mcpsdk.Tool{
			Name:        "qbit_list_tags",
			Description: "List all tags configured in qBittorrent. Use before qbit_add_download to discover existing tag names; passing an unknown tag to qbit_add_download.tags auto-creates it.",
			Annotations: readOnlyAnnotations(),
		},
		wrap("qbit_list_tags", logger, listTagsHandler(client)),
	)
}

// --- qbit_list_tags ---

type ListTagsInput struct{}

type ListTagsOutput struct {
	Tags []string `json:"tags"`
}

func listTagsHandler(client *qbt.Client) internalHandler[ListTagsInput, ListTagsOutput] {
	return func(ctx context.Context, _ ListTagsInput) (ListTagsOutput, *ToolError) {
		tags, err := client.GetTagsCtx(ctx)
		if err != nil {
			return ListTagsOutput{Tags: []string{}}, errorFromSDK(err)
		}
		if tags == nil {
			tags = []string{}
		}
		return ListTagsOutput{Tags: tags}, nil
	}
}
