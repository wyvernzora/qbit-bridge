package downloads

import (
	"context"
	"log/slog"

	qbt "github.com/autobrr/go-qbittorrent"

	"github.com/wyvernzora/qbit-bridge/internal/savepath"
)

type internalHandler[I, O any] func(ctx context.Context, in I) (O, *ToolError)

// Service owns download business logic shared by transport adapters.
type Service struct {
	search     internalHandler[SearchDownloadsInput, SearchDownloadsOutput]
	add        internalHandler[AddDownloadInput, AddDownloadOutput]
	remove     internalHandler[RemoveDownloadsInput, AffectedOutput]
	updateTags internalHandler[UpdateDownloadTagsInput, AffectedOutput]
}

func New(client *qbt.Client, resolver *savepath.Resolver, logger *slog.Logger) *Service {
	return &Service{
		search:     searchDownloadsHandler(client, resolver),
		add:        addDownloadHandler(client, resolver, logger),
		remove:     removeDownloadsHandler(client, logger),
		updateTags: updateDownloadTagsHandler(client, logger),
	}
}

// Search lists downloads using the same business logic as every transport.
func (s *Service) Search(ctx context.Context, in SearchDownloadsInput) (SearchDownloadsOutput, *ToolError) {
	return s.search(ctx, in)
}

// Add submits a magnet download request to qBittorrent.
func (s *Service) Add(ctx context.Context, in AddDownloadInput) (AddDownloadOutput, *ToolError) {
	return s.add(ctx, in)
}

// Remove forgets downloads from qBittorrent tracking without deleting files.
func (s *Service) Remove(ctx context.Context, in RemoveDownloadsInput) (AffectedOutput, *ToolError) {
	return s.remove(ctx, in)
}

// UpdateTags patches tags on explicitly selected downloads.
func (s *Service) UpdateTags(ctx context.Context, in UpdateDownloadTagsInput) (AffectedOutput, *ToolError) {
	return s.updateTags(ctx, in)
}
