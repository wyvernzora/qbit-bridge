package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	qbt "github.com/autobrr/go-qbittorrent"

	"github.com/wyvernzora/qbittorrent-mcp/internal/savepath"
)

// --- qbit_subscribe ---

// SubscribeInput creates or replaces a managed qBittorrent RSS subscription.
type SubscribeInput struct {
	Name           string   `json:"name" jsonschema:"unique subscription name; doubles as the underlying qBittorrent rule name."`
	FeedURL        string   `json:"feed_url" jsonschema:"RSS feed URL. Subscriptions sharing the same feed_url share storage transparently. Immutable for the lifetime of the subscription."`
	Enabled        *bool    `json:"enabled,omitempty" jsonschema:"enable or disable the rule; default true on create."`
	MustContain    *string  `json:"must_contain,omitempty" jsonschema:"item must contain this string or regex (depending on use_regex). For anime feeds like DMHY, regex is the workhorse — qBittorrent's native episode-filter and smart-filter assume Scene-style SxxEyy naming that anime trackers don't follow."`
	MustNotContain *string  `json:"must_not_contain,omitempty" jsonschema:"item must not contain this string or regex. Applied after must_contain."`
	UseRegex       *bool    `json:"use_regex,omitempty" jsonschema:"treat must_contain / must_not_contain as Perl-style regex. Recommended on for anime feeds."`
	Destination    string   `json:"destination,omitempty" jsonschema:"save-destination alias for matched downloads. Accepts '<alias>' for the alias root or '<alias>:<relpath>' to target a subdirectory (relpath must not start with '/' or contain '..'). Reserved name 'unspecified' rejected — output-only sentinel. Empty inherits qBittorrent's account default."`
	Tags           []string `json:"tags" jsonschema:"tags applied to every download the rule auto-adds. Required on every call. Editing on replace re-tags future matches only; existing matches keep their original tags."`
	IgnoreDays     *int     `json:"ignore_days,omitempty" jsonschema:"cool-down days between matches."`
	AddPaused      *bool    `json:"add_paused,omitempty" jsonschema:"add matched downloads in paused state."`
}

func subscribeHandler(client *qbt.Client, resolver *savepath.Resolver, logger *slog.Logger) internalHandler[SubscribeInput, OkOutput] {
	return func(ctx context.Context, in SubscribeInput) (OkOutput, *ToolError) {
		if terr := validateSubscribe(in); terr != nil {
			return OkOutput{}, terr
		}
		savePath, rerr := resolver.Resolve(in.Destination)
		if rerr != nil {
			return OkOutput{}, &ToolError{Code: CodeInvalidArgument, Message: rerr.Error(), Retriable: false}
		}

		feedPath := feedPathForURL(in.FeedURL)

		existingRules, err := client.GetRSSRulesCtx(ctx)
		if err != nil {
			return OkOutput{}, errorFromSDK(err)
		}
		if prior, ok := existingRules[in.Name]; ok && ruleIsManaged(prior) {
			if prior.AffectedFeeds[0] != feedPath {
				return OkOutput{}, &ToolError{
					Code:      CodeInvalidArgument,
					Message:   "feed_url is immutable on an existing subscription; delete and re-create to change it",
					Retriable: false,
				}
			}
		}

		feedExists, err := feedExistsAtPath(ctx, client, feedPath)
		if err != nil {
			return OkOutput{}, errorFromSDK(err)
		}

		auditMutation(ctx, logger, slog.LevelInfo, "subscribe", []string{in.Name},
			slog.String("feed_url", in.FeedURL),
			slog.String("destination", in.Destination),
			slog.Any("tags", in.Tags),
		)

		if !feedExists {
			if err := client.AddRSSFeedCtx(ctx, in.FeedURL, feedPath); err != nil {
				return OkOutput{}, errorFromSDK(err)
			}
		}

		rule := buildRule(in, feedPath, savePath)
		if err := client.SetRSSRuleCtx(ctx, in.Name, rule); err != nil {
			return OkOutput{}, errorFromSDK(err)
		}
		return OkOutput{Ok: true}, nil
	}
}

func validateSubscribe(in SubscribeInput) *ToolError {
	if strings.TrimSpace(in.Name) == "" {
		return &ToolError{Code: CodeInvalidArgument, Message: "name is required", Retriable: false}
	}
	if strings.TrimSpace(in.FeedURL) == "" {
		return &ToolError{Code: CodeInvalidArgument, Message: "feed_url is required", Retriable: false}
	}
	if in.Tags == nil {
		return &ToolError{Code: CodeInvalidArgument, Message: "tags is required (pass [] for no tags)", Retriable: false}
	}
	for _, t := range in.Tags {
		if strings.Contains(t, ",") {
			return &ToolError{
				Code:      CodeInvalidArgument,
				Message:   fmt.Sprintf("tag %q contains a comma; qBittorrent stores tags CSV-encoded so commas inside a tag would corrupt the list", t),
				Retriable: false,
			}
		}
	}
	return nil
}

// feedExistsAtPath checks whether the feed at feedPath is already
// registered in qBittorrent. Used to decide whether set_subscription
// needs to call AddRSSFeed (first time) or skip it (sharing a feed with
// another subscription). withData=false is enough — we only need the
// path-to-URL mapping, not articles.
func feedExistsAtPath(ctx context.Context, client *qbt.Client, feedPath string) (bool, error) {
	items, err := client.GetRSSItemsCtx(ctx, false)
	if err != nil {
		return false, err
	}
	_, ok := findFeedAtPath(items, feedPath)
	return ok, nil
}

func buildRule(in SubscribeInput, feedPath, savePath string) qbt.RSSAutoDownloadRule {
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	rule := qbt.RSSAutoDownloadRule{
		Enabled:       enabled,
		AffectedFeeds: []string{feedPath},
		TorrentParams: &qbt.RSSRuleTorrentParams{
			Tags:       in.Tags,
			SavePath:   savePath,
			UseAutoTMM: ptrBool(false),
		},
	}
	if in.MustContain != nil {
		rule.MustContain = *in.MustContain
	}
	if in.MustNotContain != nil {
		rule.MustNotContain = *in.MustNotContain
	}
	if in.UseRegex != nil {
		rule.UseRegex = *in.UseRegex
	}
	if in.IgnoreDays != nil {
		rule.IgnoreDays = *in.IgnoreDays
	}
	if in.AddPaused != nil {
		rule.TorrentParams.Stopped = ptrBool(*in.AddPaused)
	}
	return rule
}

func ptrBool(b bool) *bool { return &b }
