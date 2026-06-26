package downloads

import (
	"context"
	"log/slog"
)

func auditMutation(ctx context.Context, logger *slog.Logger, level slog.Level, action string, hashes []string, extra ...slog.Attr) {
	attrs := []slog.Attr{
		slog.Bool("audit", true),
		slog.String("action", action),
		slog.Any("hashes", hashes),
		slog.Int("count", len(hashes)),
	}
	attrs = append(attrs, extra...)
	logger.LogAttrs(ctx, level, "tool audit", attrs...)
}
