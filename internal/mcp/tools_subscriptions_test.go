package mcp

import (
	"context"
	"strings"
	"testing"
)

func TestFeedPathForURL_DeterministicAndPrefixed(t *testing.T) {
	got := feedPathForURL("https://example.com/feed.xml")
	if !strings.HasPrefix(got, "qbit-mcp/") {
		t.Errorf("feedPath = %q, want qbit-mcp/<hash> prefix", got)
	}
	hash := strings.TrimPrefix(got, "qbit-mcp/")
	if len(hash) != 16 {
		t.Errorf("hash len = %d, want 16", len(hash))
	}
	if got2 := feedPathForURL("https://example.com/feed.xml"); got2 != got {
		t.Errorf("non-deterministic: %q vs %q", got, got2)
	}
}

func TestFeedPathForURL_DifferentURLsDifferentPaths(t *testing.T) {
	a := feedPathForURL("https://example.com/feed.xml")
	b := feedPathForURL("https://example.com/other.xml")
	if a == b {
		t.Errorf("distinct URLs produced same path: %q", a)
	}
}

func TestSubscriptionHandlers_Stubbed(t *testing.T) {
	listH := listSubscriptionsHandler(nil)
	if _, terr := listH(context.Background(), ListSubscriptionsInput{}); terr == nil || terr.Code != CodeInternal {
		t.Errorf("list_subscriptions err = %+v, want stub internal", terr)
	}
	setH := setSubscriptionHandler(nil, nil, nil)
	if _, terr := setH(context.Background(), SetSubscriptionInput{}); terr == nil || terr.Code != CodeInternal {
		t.Errorf("set_subscription err = %+v, want stub internal", terr)
	}
	delH := deleteSubscriptionHandler(nil, nil)
	if _, terr := delH(context.Background(), DeleteSubscriptionInput{}); terr == nil || terr.Code != CodeInternal {
		t.Errorf("delete_subscription err = %+v, want stub internal", terr)
	}
}
