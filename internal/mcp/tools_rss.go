package mcp

import (
	"context"
	"log/slog"

	qbt "github.com/autobrr/go-qbittorrent"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wyvernzora/qbittorrent-mcp/internal/savepath"
)

// registerRSS wires the 6 RSS tools onto s. The handlers reach qBittorrent's
// /api/v2/rss/* endpoints directly via client.GetHTTPClient() because the
// autobrr/go-qbittorrent SDK does not surface them as of v1.15.0. The bypass
// helper will live under internal/qbtrss/ when the handlers are implemented.
// Tracking issue (file upstream): TODO.
func registerRSS(s *mcpsdk.Server, client *qbt.Client, resolver *savepath.Resolver, logger *slog.Logger) {
	destHint := resolver.DescriptionHint()

	mcpsdk.AddTool(s,
		&mcpsdk.Tool{
			Name:        "list_rss",
			Description: "List RSS feeds and folders. Feed paths are flattened with '/' as the folder separator (e.g. 'Anime/Erai-raws/Erai-raws Releases'). Items are omitted by default because feeds can carry hundreds; set include_items=true and use since to window the result.",
			Annotations: readOnlyAnnotations(),
		},
		wrap("list_rss", logger, listRSSHandler(client)),
	)
	mcpsdk.AddTool(s,
		&mcpsdk.Tool{
			Name:        "add_rss_feed",
			Description: "Subscribe to a new RSS feed at the given flat path. Parent folders in the path are created as needed.",
			Annotations: mutatingAnnotations(false),
		},
		wrap("add_rss_feed", logger, addRSSFeedHandler(client)),
	)
	mcpsdk.AddTool(s,
		&mcpsdk.Tool{
			Name:        "remove_rss_item",
			Description: "Remove a feed or folder at the given flat path. qBittorrent uses the same endpoint for both — pass the folder path to remove the folder and all feeds under it.",
			Annotations: mutatingAnnotations(true),
		},
		wrap("remove_rss_item", logger, removeRSSItemHandler(client)),
	)
	mcpsdk.AddTool(s,
		&mcpsdk.Tool{
			Name:        "list_rss_rules",
			Description: "List auto-download rules. Each rule names the feeds it filters, its match/exclude strings, episode filter, and the save_path / destination matched downloads are added to.",
			Annotations: readOnlyAnnotations(),
		},
		wrap("list_rss_rules", logger, listRSSRulesHandler(client)),
	)
	mcpsdk.AddTool(s,
		&mcpsdk.Tool{
			Name:        "set_rss_rule",
			Description: "Upsert an auto-download rule by name. qBittorrent's setRule endpoint is create-or-replace — same payload either way. Fields you do not pass keep their defaults on create or current values on edit. " + destHint,
			Annotations: mutatingAnnotations(false),
		},
		wrap("set_rss_rule", logger, setRSSRuleHandler(client, resolver)),
	)
	mcpsdk.AddTool(s,
		&mcpsdk.Tool{
			Name:        "delete_rss_rule",
			Description: "Remove an auto-download rule by name.",
			Annotations: mutatingAnnotations(true),
		},
		wrap("delete_rss_rule", logger, deleteRSSRuleHandler(client)),
	)
}

// --- list_rss ---

type ListRSSInput struct {
	IncludeItems bool   `json:"include_items,omitempty" jsonschema:"include items inside each feed. default false because feeds can carry hundreds of entries."`
	Since        string `json:"since,omitempty" jsonschema:"RFC3339 timestamp; with include_items=true, only items with pub_date >= since are returned."`
}

type RSSFeedItem struct {
	Title        string `json:"title"`
	Link         string `json:"link"`
	PubDate      string `json:"pub_date"`
	MatchingRule string `json:"matching_rule,omitempty"`
}

type RSSFeed struct {
	Path          string        `json:"path"`
	URL           string        `json:"url"`
	HasError      bool          `json:"has_error"`
	LastBuildDate string        `json:"last_build_date,omitempty"`
	ItemCount     int           `json:"item_count"`
	Items         []RSSFeedItem `json:"items,omitempty"`
}

type ListRSSOutput struct {
	Feeds []RSSFeed `json:"feeds"`
}

func listRSSHandler(_ *qbt.Client) internalHandler[ListRSSInput, ListRSSOutput] {
	return func(_ context.Context, _ ListRSSInput) (ListRSSOutput, *ToolError) {
		return ListRSSOutput{Feeds: []RSSFeed{}}, notImplemented("list_rss")
	}
}

// --- add_rss_feed ---

type AddRSSFeedInput struct {
	URL  string `json:"url" jsonschema:"RSS feed URL"`
	Path string `json:"path" jsonschema:"flat slash-separated path inside qBittorrent's RSS tree, e.g. 'Anime/Erai-raws/Erai-raws Releases'"`
}

func addRSSFeedHandler(_ *qbt.Client) internalHandler[AddRSSFeedInput, OkOutput] {
	return func(_ context.Context, _ AddRSSFeedInput) (OkOutput, *ToolError) {
		return OkOutput{}, notImplemented("add_rss_feed")
	}
}

// --- remove_rss_item ---

type RemoveRSSItemInput struct {
	Path string `json:"path" jsonschema:"flat slash-separated path of the feed or folder to remove"`
}

func removeRSSItemHandler(_ *qbt.Client) internalHandler[RemoveRSSItemInput, OkOutput] {
	return func(_ context.Context, _ RemoveRSSItemInput) (OkOutput, *ToolError) {
		return OkOutput{}, notImplemented("remove_rss_item")
	}
}

// --- list_rss_rules ---

type ListRSSRulesInput struct{}

type RSSRule struct {
	Name           string   `json:"name"`
	Enabled        bool     `json:"enabled"`
	MustContain    string   `json:"must_contain,omitempty"`
	MustNotContain string   `json:"must_not_contain,omitempty"`
	UseRegex       bool     `json:"use_regex"`
	EpisodeFilter  string   `json:"episode_filter,omitempty"`
	SmartFilter    bool     `json:"smart_filter"`
	AffectedFeeds  []string `json:"affected_feeds"`
	SavePath       string   `json:"save_path,omitempty"`
	IgnoreDays     int      `json:"ignore_days"`
	AddPaused      bool     `json:"add_paused"`
}

type ListRSSRulesOutput struct {
	Rules []RSSRule `json:"rules"`
}

func listRSSRulesHandler(_ *qbt.Client) internalHandler[ListRSSRulesInput, ListRSSRulesOutput] {
	return func(_ context.Context, _ ListRSSRulesInput) (ListRSSRulesOutput, *ToolError) {
		return ListRSSRulesOutput{Rules: []RSSRule{}}, notImplemented("list_rss_rules")
	}
}

// --- set_rss_rule ---

type SetRSSRuleInput struct {
	Name           string   `json:"name" jsonschema:"unique rule name"`
	Enabled        *bool    `json:"enabled,omitempty" jsonschema:"enable or disable the rule; default true on create"`
	MustContain    *string  `json:"must_contain,omitempty" jsonschema:"item must contain this string/regex"`
	MustNotContain *string  `json:"must_not_contain,omitempty" jsonschema:"item must not contain this string/regex"`
	UseRegex       *bool    `json:"use_regex,omitempty" jsonschema:"treat must_contain / must_not_contain as regex"`
	EpisodeFilter  *string  `json:"episode_filter,omitempty" jsonschema:"qBittorrent episode-filter expression, e.g. '1x2;'"`
	SmartFilter    *bool    `json:"smart_filter,omitempty" jsonschema:"qBittorrent's deduplicating smart filter"`
	AffectedFeeds  []string `json:"affected_feeds,omitempty" jsonschema:"flat feed paths this rule applies to"`
	Destination    *string  `json:"destination,omitempty" jsonschema:"save-destination alias name for matched downloads. See the tool description for the valid set."`
	IgnoreDays     *int     `json:"ignore_days,omitempty" jsonschema:"cool-down days between matches"`
	AddPaused      *bool    `json:"add_paused,omitempty" jsonschema:"add matched downloads in paused state"`
}

func setRSSRuleHandler(_ *qbt.Client, _ *savepath.Resolver) internalHandler[SetRSSRuleInput, OkOutput] {
	return func(_ context.Context, _ SetRSSRuleInput) (OkOutput, *ToolError) {
		return OkOutput{}, notImplemented("set_rss_rule")
	}
}

// --- delete_rss_rule ---

type DeleteRSSRuleInput struct {
	Name string `json:"name" jsonschema:"rule name to remove"`
}

func deleteRSSRuleHandler(_ *qbt.Client) internalHandler[DeleteRSSRuleInput, OkOutput] {
	return func(_ context.Context, _ DeleteRSSRuleInput) (OkOutput, *ToolError) {
		return OkOutput{}, notImplemented("delete_rss_rule")
	}
}
