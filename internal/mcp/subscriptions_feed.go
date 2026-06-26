package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	qbt "github.com/autobrr/go-qbittorrent"
)

const (
	// feedPathPrefix is the qBittorrent RSS feed-name prefix for every
	// feed created by qbit-mcp. Flat (no folder) — keeps the synthetic
	// path single-token, so the RSS folder-separator question (qbit uses
	// backslash) never enters play.
	feedPathPrefix = "qbit-mcp-"
)

// feedPathForURL derives the synthetic qBittorrent RSS feed path for a
// given feed URL. Subscriptions that share a feed_url collide on this
// path, which is the dedupe mechanism — qBittorrent stores the feed once
// and multiple rules can reference it. The 16-hex-char prefix of the
// sha256 is enough collision resistance for the cardinalities a single
// qBittorrent instance handles in practice while staying short enough to
// browse comfortably in qbit's WebUI tree.
//
// URL is the literal dedupe key; no normalization is applied. Callers
// are responsible for using a consistent URL form.
func feedPathForURL(url string) string {
	sum := sha256.Sum256([]byte(url))
	return feedPathPrefix + hex.EncodeToString(sum[:])[:16]
}

// ruleIsManaged tests whether a qBittorrent rule belongs to the
// subscription surface qbit-mcp exposes. We only surface rules with
// exactly one affected feed whose path is under the synthetic
// "qbit-mcp-" prefix; rules created via qBittorrent's WebUI directly,
// or rules targeting multiple feeds, are deliberately invisible.
func ruleIsManaged(rule qbt.RSSAutoDownloadRule) bool {
	if len(rule.AffectedFeeds) != 1 {
		return false
	}
	return strings.HasPrefix(rule.AffectedFeeds[0], feedPathPrefix)
}

// findFeedAtPath walks the hierarchical RSSItems map looking for a feed
// at the given slash-or-backslash-separated path. Returns the feed and
// ok=true when found. The path uses qBittorrent's native separator
// (backslash) but for our flat top-level entries either separator works
// because there is only one path component.
func findFeedAtPath(items qbt.RSSItems, path string) (qbt.RSSFeed, bool) {
	parts := splitFeedPath(path)
	cursor := items
	for i, part := range parts {
		raw, ok := cursor[part]
		if !ok {
			return qbt.RSSFeed{}, false
		}
		if i == len(parts)-1 {
			var feed qbt.RSSFeed
			if err := json.Unmarshal(raw, &feed); err == nil && feed.URL != "" {
				return feed, true
			}
			return qbt.RSSFeed{}, false
		}
		var nested qbt.RSSItems
		if err := json.Unmarshal(raw, &nested); err != nil {
			return qbt.RSSFeed{}, false
		}
		cursor = nested
	}
	return qbt.RSSFeed{}, false
}

// splitFeedPath handles both qBittorrent's native backslash separator
// and the forward-slash form some operator-side tools use. qbit-mcp's
// own feed paths are flat single-token strings so this is mostly a
// safety net.
func splitFeedPath(path string) []string {
	switch {
	case strings.Contains(path, `\`):
		return strings.Split(path, `\`)
	case strings.Contains(path, "/"):
		return strings.Split(path, "/")
	default:
		return []string{path}
	}
}

// feedStillReferenced reports whether any managed rule other than the
// one we just removed (excludeName) still points at feedPath. Used by
// delete to decide whether to garbage-collect the synthetic feed.
func feedStillReferenced(rules qbt.RSSRules, excludeName, feedPath string) bool {
	for name, r := range rules {
		if name == excludeName {
			continue
		}
		if !ruleIsManaged(r) {
			continue
		}
		if r.AffectedFeeds[0] == feedPath {
			return true
		}
	}
	return false
}
