package mcp

import (
	"log/slog"

	qbt "github.com/autobrr/go-qbittorrent"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wyvernzora/qbit-bridge/internal/savepath"
)

// registerSubscriptions wires the 3 subscription tools onto s. A
// subscription bundles a qBittorrent RSS feed and the auto-download rule
// that filters its items into actual downloads — the two-layer model
// (feeds, rules) is fused so agents work with a single concept: "watch
// this URL, add matches to this destination with these tags".
//
// All handlers reach qBittorrent through the autobrr SDK
// (github.com/autobrr/go-qbittorrent v1.15.0), which covers the
// /api/v2/rss/* endpoints we need. No direct HTTP fallback is required.
func registerSubscriptions(s *mcpsdk.Server, client *qbt.Client, resolver *savepath.Resolver, logger *slog.Logger) {
	destHint := resolver.DescriptionHint()

	mcpsdk.AddTool(s,
		&mcpsdk.Tool{
			Name:        "qbit_search_subscriptions",
			Description: "Search subscriptions with optional name/feed_url filters and pagination. Each row carries the feed URL, the rule's filter fields (must_contain, must_not_contain, use_regex), the destination alias the rule routes matched downloads to, the tags applied at creation, and a last_match_date summary. Set include_recent_items=true to embed the most-recent feed items (capped by recent_items_limit, default 10, max 50). Default limit 50, max 200; paginate via offset.",
			Annotations: readOnlyAnnotations(),
		},
		wrap("qbit_search_subscriptions", logger, searchSubscriptionsHandler(client, resolver)),
	)
	mcpsdk.AddTool(s,
		&mcpsdk.Tool{
			Name:        "qbit_subscribe",
			Description: "Create or replace a subscription by name. Atomically creates (or replaces) the qBittorrent feed and the auto-download rule pointing at it. The feed_url is the only feed-side input; qbit-bridge derives a synthetic feed path 'qbit-bridge-<hash>' so duplicate feed_urls across subscriptions share storage transparently. Changing feed_url on an existing subscription is rejected — unsubscribe and resubscribe instead. tags is required on every call; passing a different tags array on replace re-tags FUTURE auto-added downloads only (existing matches keep their original tags — retroactive retag is out of scope). " + destHint,
			Annotations: mutatingAnnotations(false),
		},
		wrap("qbit_subscribe", logger, subscribeHandler(client, resolver, logger)),
	)
	mcpsdk.AddTool(s,
		&mcpsdk.Tool{
			Name:        "qbit_unsubscribe",
			Description: "Unsubscribe by name. Removes the auto-download rule; the underlying feed is removed too unless another subscription still references the same feed_url.",
			Annotations: mutatingAnnotations(true),
		},
		wrap("qbit_unsubscribe", logger, unsubscribeHandler(client, logger)),
	)
}
