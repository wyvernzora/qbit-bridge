package mcp

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	qbt "github.com/autobrr/go-qbittorrent"

	"github.com/wyvernzora/qbit-bridge/internal/savepath"
)

const (
	defaultRecentItemsLimit = 10
	maxRecentItemsLimit     = 50

	defaultSearchSubscriptionsLimit = 50
	maxSearchSubscriptionsLimit     = 200
)

// --- qbit_search_subscriptions ---

// SearchSubscriptionsInput filters, paginates, and projects subscriptions.
type SearchSubscriptionsInput struct {
	NameGlob           string `json:"name_glob,omitempty" jsonschema:"optional shell-style glob (path.Match) on subscription name; '*', '?', '[abc]' supported. plain strings match exactly."`
	FeedURLSubstring   string `json:"feed_url_substring,omitempty" jsonschema:"optional case-sensitive substring filter on feed_url."`
	Limit              int    `json:"limit,omitempty" jsonschema:"max subscriptions to return; default 50, max 200."`
	Offset             int    `json:"offset,omitempty" jsonschema:"page offset; default 0."`
	IncludeRecentItems bool   `json:"include_recent_items,omitempty" jsonschema:"embed each subscription's most-recent feed items. default false because feeds can carry hundreds of entries."`
	RecentItemsLimit   int    `json:"recent_items_limit,omitempty" jsonschema:"max items per subscription when include_recent_items=true. default 10, max 50."`
	Since              string `json:"since,omitempty" jsonschema:"RFC3339 timestamp; with include_recent_items=true, only items with pub_date >= since are returned."`
}

// SubscriptionItem is a recent RSS article attached to a projected subscription.
type SubscriptionItem struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	PubDate string `json:"pub_date"`
}

// Subscription is the MCP projection of a qBittorrent RSS auto-download rule.
type Subscription struct {
	Name           string             `json:"name"`
	FeedURL        string             `json:"feed_url"`
	Enabled        bool               `json:"enabled"`
	MustContain    string             `json:"must_contain,omitempty"`
	MustNotContain string             `json:"must_not_contain,omitempty"`
	UseRegex       bool               `json:"use_regex"`
	SavePath       string             `json:"save_path,omitempty"`
	Tags           []string           `json:"tags"`
	IgnoreDays     int                `json:"ignore_days"`
	AddPaused      bool               `json:"add_paused"`
	FeedHasError   bool               `json:"feed_has_error"`
	LastMatchDate  string             `json:"last_match_date,omitempty"`
	RecentItems    []SubscriptionItem `json:"recent_items,omitempty"`
}

// SearchSubscriptionsOutput is the paginated subscription search response.
type SearchSubscriptionsOutput struct {
	Count         int            `json:"count"`
	HasMore       bool           `json:"has_more"`
	Subscriptions []Subscription `json:"subscriptions"`
}

// searchSubscriptionsRequest is the validated, ready-to-execute form of
// a SearchSubscriptionsInput. prepareSearchSubscriptions produces it
// after every validation rule, keeping the handler body thin (and
// inside the cyclop budget).
type searchSubscriptionsRequest struct {
	in          SearchSubscriptionsInput
	pageLimit   int
	pageOffset  int
	recentLimit int
	sinceCutoff time.Time
}

func searchSubscriptionsHandler(client *qbt.Client, resolver *savepath.Resolver) internalHandler[SearchSubscriptionsInput, SearchSubscriptionsOutput] {
	return func(ctx context.Context, in SearchSubscriptionsInput) (SearchSubscriptionsOutput, *ToolError) {
		empty := SearchSubscriptionsOutput{Subscriptions: []Subscription{}}

		req, terr := prepareSearchSubscriptions(in)
		if terr != nil {
			return empty, terr
		}

		rules, err := client.GetRSSRulesCtx(ctx)
		if err != nil {
			return empty, errorFromSDK(err)
		}
		// withData=true returns inline article arrays, which we need
		// regardless of include_recent_items because last_match_date
		// already comes from rule.LastMatch and recent_items needs the
		// articles. The single fetch covers both.
		items, err := client.GetRSSItemsCtx(ctx, in.IncludeRecentItems)
		if err != nil {
			return empty, errorFromSDK(err)
		}

		filtered := projectAndFilterSubscriptions(rules, items, req, resolver)
		page, hasMore := paginateSubscriptions(filtered, req.pageOffset, req.pageLimit)
		return SearchSubscriptionsOutput{
			Count:         len(page),
			HasMore:       hasMore,
			Subscriptions: page,
		}, nil
	}
}

// prepareSearchSubscriptions validates input and normalizes the
// derived defaults (pageLimit/Offset, recentLimit, sinceCutoff).
// Catches name_glob syntax errors and malformed since timestamps up
// front so callers see invalid_argument rather than silent zero-result
// pages.
func prepareSearchSubscriptions(in SearchSubscriptionsInput) (searchSubscriptionsRequest, *ToolError) {
	if in.NameGlob != "" {
		if _, err := path.Match(in.NameGlob, ""); err != nil {
			return searchSubscriptionsRequest{}, &ToolError{
				Code:      CodeInvalidArgument,
				Message:   fmt.Sprintf("invalid name_glob %q: %v", in.NameGlob, err),
				Retriable: false,
			}
		}
	}
	pageLimit, terr := normalizeSearchLimit(in.Limit)
	if terr != nil {
		return searchSubscriptionsRequest{}, terr
	}
	pageOffset, terr := normalizeSearchOffset(in.Offset)
	if terr != nil {
		return searchSubscriptionsRequest{}, terr
	}
	recentLimit, terr := normalizeRecentItemsLimit(in.RecentItemsLimit)
	if terr != nil {
		return searchSubscriptionsRequest{}, terr
	}
	var sinceCutoff time.Time
	if in.Since != "" {
		t, err := time.Parse(time.RFC3339, in.Since)
		if err != nil {
			return searchSubscriptionsRequest{}, &ToolError{
				Code:      CodeInvalidArgument,
				Message:   "since must be RFC3339: " + err.Error(),
				Retriable: false,
			}
		}
		sinceCutoff = t
	}
	return searchSubscriptionsRequest{
		in:          in,
		pageLimit:   pageLimit,
		pageOffset:  pageOffset,
		recentLimit: recentLimit,
		sinceCutoff: sinceCutoff,
	}, nil
}

// projectAndFilterSubscriptions walks the managed rules in
// alphabetical order, applies the name/feed_url filters, and projects
// each survivor into the wire shape. Items projection happens here so
// pagination operates on the filtered+projected set (not the raw rule
// list — a name_glob with a high offset shouldn't return an empty
// page just because unmatched rules occupied the early indices).
func projectAndFilterSubscriptions(
	rules qbt.RSSRules,
	items qbt.RSSItems,
	req searchSubscriptionsRequest,
	resolver *savepath.Resolver,
) []Subscription {
	names := make([]string, 0, len(rules))
	for name := range rules {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]Subscription, 0, len(names))
	for _, name := range names {
		rule := rules[name]
		if !ruleIsManaged(rule) {
			continue
		}
		feedPath := rule.AffectedFeeds[0]
		feed, _ := findFeedAtPath(items, feedPath)
		if req.in.NameGlob != "" {
			if ok, _ := path.Match(req.in.NameGlob, name); !ok {
				continue
			}
		}
		if req.in.FeedURLSubstring != "" && !strings.Contains(feed.URL, req.in.FeedURLSubstring) {
			continue
		}
		sub := projectSubscription(name, rule, feed, resolver)
		if req.in.IncludeRecentItems {
			sub.RecentItems = projectRecentItems(feed.Articles, req.recentLimit, req.sinceCutoff)
		}
		out = append(out, sub)
	}
	return out
}

// paginateSubscriptions slices the filtered set per offset+limit and
// reports whether more entries exist past the returned page. Mirrors
// paginateDownloads but on the Subscription value type.
func paginateSubscriptions(filtered []Subscription, offset, limit int) ([]Subscription, bool) {
	total := len(filtered)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return filtered[start:end], end < total
}

func projectSubscription(name string, rule qbt.RSSAutoDownloadRule, feed qbt.RSSFeed, resolver *savepath.Resolver) Subscription {
	savePath, tags, addPaused := extractRuleParams(rule)
	out := Subscription{
		Name:           name,
		FeedURL:        feed.URL,
		Enabled:        rule.Enabled,
		MustContain:    rule.MustContain,
		MustNotContain: rule.MustNotContain,
		UseRegex:       rule.UseRegex,
		SavePath:       prefixed(resolver, savePath),
		Tags:           tags,
		IgnoreDays:     rule.IgnoreDays,
		AddPaused:      addPaused,
		FeedHasError:   feed.HasError,
		LastMatchDate:  rule.LastMatch,
	}
	return out
}

// extractRuleParams pulls the save_path / tags / add_paused values out
// of the rule, preferring the modern TorrentParams shape and falling
// back to the legacy top-level fields qBittorrent still emits on older
// installs.
func extractRuleParams(rule qbt.RSSAutoDownloadRule) (savePath string, tags []string, addPaused bool) {
	tags = []string{}
	if rule.TorrentParams != nil {
		savePath = rule.TorrentParams.SavePath
		if rule.TorrentParams.Tags != nil {
			tags = rule.TorrentParams.Tags
		}
		if rule.TorrentParams.Stopped != nil {
			addPaused = *rule.TorrentParams.Stopped
		}
	}
	if savePath == "" {
		savePath = rule.SavePath
	}
	if !addPaused && rule.AddPaused != nil {
		addPaused = *rule.AddPaused
	}
	return savePath, tags, addPaused
}

func projectRecentItems(articles []qbt.RSSArticle, limit int, since time.Time) []SubscriptionItem {
	out := make([]SubscriptionItem, 0, limit)
	for _, a := range articles {
		if !since.IsZero() {
			t, err := parseArticleDate(a.Date)
			if err != nil || t.Before(since) {
				continue
			}
		}
		out = append(out, SubscriptionItem{
			Title:   a.Title,
			Link:    pickArticleLink(a),
			PubDate: a.Date,
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

// pickArticleLink prefers the torrent URL (magnet or .torrent) over the
// HTML link; the magnet form is what the rule will actually feed into
// qBittorrent on a match, so it is the more useful thing to surface.
func pickArticleLink(a qbt.RSSArticle) string {
	if a.TorrentURL != "" {
		return a.TorrentURL
	}
	return a.Link
}

// parseArticleDate accepts the date formats qBittorrent emits across
// versions. ISO 8601 with offset is the modern form; RFC1123Z appears on
// older builds. Failures fall through to the caller which treats them
// as "skip the since filter for this article".
func parseArticleDate(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", time.RFC1123Z, time.RFC1123} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date %q", s)
}

func normalizeRecentItemsLimit(in int) (int, *ToolError) {
	if in < 0 {
		return 0, &ToolError{Code: CodeInvalidArgument, Message: "recent_items_limit must be >= 0", Retriable: false}
	}
	if in == 0 {
		return defaultRecentItemsLimit, nil
	}
	if in > maxRecentItemsLimit {
		return maxRecentItemsLimit, nil
	}
	return in, nil
}

func normalizeSearchLimit(in int) (int, *ToolError) {
	if in < 0 {
		return 0, &ToolError{Code: CodeInvalidArgument, Message: "limit must be >= 0", Retriable: false}
	}
	if in == 0 {
		return defaultSearchSubscriptionsLimit, nil
	}
	if in > maxSearchSubscriptionsLimit {
		return maxSearchSubscriptionsLimit, nil
	}
	return in, nil
}

func normalizeSearchOffset(in int) (int, *ToolError) {
	if in < 0 {
		return 0, &ToolError{Code: CodeInvalidArgument, Message: "offset must be >= 0", Retriable: false}
	}
	return in, nil
}

func prefixed(resolver *savepath.Resolver, path string) string {
	if path == "" {
		return ""
	}
	return resolver.NameForPathPrefixed(path)
}
