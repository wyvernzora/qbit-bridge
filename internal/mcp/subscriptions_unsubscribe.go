package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	qbt "github.com/autobrr/go-qbittorrent"
)

// --- qbit_unsubscribe ---

// UnsubscribeInput removes a managed qBittorrent RSS subscription by name.
type UnsubscribeInput struct {
	Name string `json:"name" jsonschema:"name of the subscription to remove."`
}

func unsubscribeHandler(client *qbt.Client, logger *slog.Logger) internalHandler[UnsubscribeInput, OkOutput] {
	return func(ctx context.Context, in UnsubscribeInput) (OkOutput, *ToolError) {
		if strings.TrimSpace(in.Name) == "" {
			return OkOutput{}, &ToolError{Code: CodeInvalidArgument, Message: "name is required", Retriable: false}
		}
		rules, err := client.GetRSSRulesCtx(ctx)
		if err != nil {
			return OkOutput{}, errorFromSDK(err)
		}
		rule, ok := rules[in.Name]
		if !ok || !ruleIsManaged(rule) {
			return OkOutput{}, &ToolError{
				Code:      CodeUpstreamNotFound,
				Message:   fmt.Sprintf("subscription %q not found", in.Name),
				Retriable: false,
			}
		}
		feedPath := rule.AffectedFeeds[0]

		auditMutation(ctx, logger, slog.LevelWarn, "unsubscribe", []string{in.Name},
			slog.String("feed_path", feedPath),
		)

		if err := client.RemoveRSSRuleCtx(ctx, in.Name); err != nil {
			return OkOutput{}, errorFromSDK(err)
		}
		if !feedStillReferenced(rules, in.Name, feedPath) {
			if err := client.RemoveRSSItemCtx(ctx, feedPath); err != nil {
				return OkOutput{}, errorFromSDK(err)
			}
		}
		return OkOutput{Ok: true}, nil
	}
}
