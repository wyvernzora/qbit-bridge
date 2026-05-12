package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"

	qbt "github.com/autobrr/go-qbittorrent"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wyvernzora/qbittorrent-mcp/internal/savepath"
)

// registerSubscriptions wires the 3 subscription tools onto s. A
// subscription bundles a qBittorrent RSS feed and the auto-download rule
// that filters its items into actual downloads — the two-layer model
// (feeds, rules) is fused so agents work with a single concept: "watch
// this URL, add matches to this destination with these tags".
//
// The handlers reach qBittorrent's /api/v2/rss/* endpoints directly via
// client.GetHTTPClient() because the autobrr/go-qbittorrent SDK does not
// surface them as of v1.15.0. The bypass helper will live under
// internal/qbtrss/ when the handlers are implemented. Tracking issue
// (file upstream): TODO.
func registerSubscriptions(s *mcpsdk.Server, client *qbt.Client, resolver *savepath.Resolver, logger *slog.Logger) {
	destHint := resolver.DescriptionHint()

	mcpsdk.AddTool(s,
		&mcpsdk.Tool{
			Name:        "list_subscriptions",
			Description: "List subscriptions. Each row carries the feed URL, the rule's filter fields (must_contain, episode_filter, ...), the destination alias the rule routes matched downloads to, the tags applied at creation, and a last_match_date / match_count summary. Set include_recent_items=true to also embed the most-recent feed items (capped by recent_items_limit, default 10, max 50).",
			Annotations: readOnlyAnnotations(),
		},
		wrap("list_subscriptions", logger, listSubscriptionsHandler(client)),
	)
	mcpsdk.AddTool(s,
		&mcpsdk.Tool{
			Name:        "set_subscription",
			Description: "Upsert a subscription by name. Atomically creates (or replaces) the qBittorrent feed and the auto-download rule pointing at it. The feed_url is the only feed-side input; qbit-mcp derives a synthetic feed path under 'qbit-mcp/<hash>' so duplicate feed_urls across subscriptions share storage transparently. Changing feed_url on an existing subscription is rejected — delete and re-create instead. tags is required on every call; passing a different tags array on replace re-tags FUTURE auto-added downloads only (existing matches keep their original tags — retroactive retag is out of scope). " + destHint,
			Annotations: mutatingAnnotations(false),
		},
		wrap("set_subscription", logger, setSubscriptionHandler(client, resolver, logger)),
	)
	mcpsdk.AddTool(s,
		&mcpsdk.Tool{
			Name:        "delete_subscription",
			Description: "Delete a subscription by name. Removes the auto-download rule; the underlying feed is removed too unless another subscription still references the same feed_url.",
			Annotations: mutatingAnnotations(true),
		},
		wrap("delete_subscription", logger, deleteSubscriptionHandler(client, logger)),
	)
}

// feedPathForURL derives the synthetic qBittorrent RSS feed path for a
// given feed URL. Subscriptions that share a feed_url collide on this
// path, which is the dedupe mechanism — qBittorrent stores the feed once
// and multiple rules can reference it. The 16-hex-char prefix of the
// sha256 is enough collision resistance for the cardinalities a single
// qBittorrent instance handles in practice while staying short enough to
// browse comfortably in qbit's WebUI tree.
func feedPathForURL(url string) string {
	sum := sha256.Sum256([]byte(url))
	return "qbit-mcp/" + hex.EncodeToString(sum[:])[:16]
}

// --- list_subscriptions ---

type ListSubscriptionsInput struct {
	IncludeRecentItems bool   `json:"include_recent_items,omitempty" jsonschema:"embed each subscription's most-recent feed items. default false because feeds can carry hundreds of entries."`
	RecentItemsLimit   int    `json:"recent_items_limit,omitempty" jsonschema:"max items per subscription when include_recent_items=true. default 10, max 50."`
	Since              string `json:"since,omitempty" jsonschema:"RFC3339 timestamp; with include_recent_items=true, only items with pub_date >= since are returned."`
}

type SubscriptionItem struct {
	Title        string `json:"title"`
	Link         string `json:"link"`
	PubDate      string `json:"pub_date"`
	MatchingRule string `json:"matching_rule,omitempty"`
}

type Subscription struct {
	Name           string             `json:"name"`
	FeedURL        string             `json:"feed_url"`
	Enabled        bool               `json:"enabled"`
	MustContain    string             `json:"must_contain,omitempty"`
	MustNotContain string             `json:"must_not_contain,omitempty"`
	UseRegex       bool               `json:"use_regex"`
	EpisodeFilter  string             `json:"episode_filter,omitempty"`
	SmartFilter    bool               `json:"smart_filter"`
	Destination    string             `json:"destination,omitempty"`
	SavePath       string             `json:"save_path,omitempty"`
	Tags           []string           `json:"tags"`
	IgnoreDays     int                `json:"ignore_days"`
	AddPaused      bool               `json:"add_paused"`
	FeedHasError   bool               `json:"feed_has_error"`
	LastMatchDate  string             `json:"last_match_date,omitempty"`
	MatchCount     int                `json:"match_count"`
	RecentItems    []SubscriptionItem `json:"recent_items,omitempty"`
}

type ListSubscriptionsOutput struct {
	Subscriptions []Subscription `json:"subscriptions"`
}

func listSubscriptionsHandler(_ *qbt.Client) internalHandler[ListSubscriptionsInput, ListSubscriptionsOutput] {
	return func(_ context.Context, _ ListSubscriptionsInput) (ListSubscriptionsOutput, *ToolError) {
		return ListSubscriptionsOutput{Subscriptions: []Subscription{}}, notImplemented("list_subscriptions")
	}
}

// --- set_subscription ---

type SetSubscriptionInput struct {
	Name           string   `json:"name" jsonschema:"unique subscription name; doubles as the underlying qBittorrent rule name."`
	FeedURL        string   `json:"feed_url" jsonschema:"RSS feed URL. Subscriptions sharing the same feed_url share storage transparently."`
	Enabled        *bool    `json:"enabled,omitempty" jsonschema:"enable or disable the rule; default true on create."`
	MustContain    *string  `json:"must_contain,omitempty" jsonschema:"item must contain this string or regex."`
	MustNotContain *string  `json:"must_not_contain,omitempty" jsonschema:"item must not contain this string or regex."`
	UseRegex       *bool    `json:"use_regex,omitempty" jsonschema:"treat must_contain / must_not_contain as regex."`
	EpisodeFilter  *string  `json:"episode_filter,omitempty" jsonschema:"qBittorrent episode-filter expression, e.g. '1x2;'."`
	SmartFilter    *bool    `json:"smart_filter,omitempty" jsonschema:"qBittorrent's deduplicating smart filter."`
	Destination    string   `json:"destination,omitempty" jsonschema:"save-destination alias name for matched downloads. Empty inherits qBittorrent's account default."`
	Tags           []string `json:"tags" jsonschema:"tags applied to every download the rule auto-adds. Required on every call. Editing on replace re-tags future matches only; existing matches keep their original tags."`
	IgnoreDays     *int     `json:"ignore_days,omitempty" jsonschema:"cool-down days between matches."`
	AddPaused      *bool    `json:"add_paused,omitempty" jsonschema:"add matched downloads in paused state."`
}

func setSubscriptionHandler(_ *qbt.Client, _ *savepath.Resolver, _ *slog.Logger) internalHandler[SetSubscriptionInput, OkOutput] {
	return func(_ context.Context, _ SetSubscriptionInput) (OkOutput, *ToolError) {
		return OkOutput{}, notImplemented("set_subscription")
	}
}

// --- delete_subscription ---

type DeleteSubscriptionInput struct {
	Name string `json:"name" jsonschema:"name of the subscription to remove."`
}

func deleteSubscriptionHandler(_ *qbt.Client, _ *slog.Logger) internalHandler[DeleteSubscriptionInput, OkOutput] {
	return func(_ context.Context, _ DeleteSubscriptionInput) (OkOutput, *ToolError) {
		return OkOutput{}, notImplemented("delete_subscription")
	}
}
