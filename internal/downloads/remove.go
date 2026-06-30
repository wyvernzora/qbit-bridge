package downloads

import (
	"context"
	"log/slog"

	qbt "github.com/autobrr/go-qbittorrent"
)

// --- remove_downloads ---

// HashesOrFilter is the bulk-op selector: pass either Hashes or Filter,
// never both, never neither. resolveTargets returns invalid_argument when
// the rule is violated.
type HashesOrFilter struct {
	Hashes []string        `json:"hashes,omitempty" jsonschema:"explicit set of download hashes to operate on"`
	Filter *FilterCriteria `json:"filter,omitempty" jsonschema:"select targets by state/tags instead of explicit hashes. mutually exclusive with hashes."`
}

// resolveTargets validates a HashesOrFilter selector and resolves it to a
// concrete hash list. For the filter path it pre-fetches every download
// and applies the same client-side filter logic search_downloads uses.
//
// Returns an empty slice (not an error) when either selector resolves to
// zero targets — an explicitly-empty hashes array, or a filter that
// matches nothing. Caller short-circuits before the upstream mutation
// and returns affected_count=0, which is the agent-friendly response
// for defensive cleanup loops that build the hash list dynamically.
//
// Rejected: passing both hashes and filter, or passing neither (which
// would be ambiguous between "operate on everything" and "agent forgot
// to include the selector"; we refuse rather than guess).
func resolveTargets(ctx context.Context, client *qbt.Client, sel HashesOrFilter) ([]string, *ToolError) {
	// Distinguish "field was explicitly provided" from "field was
	// absent" by checking the underlying slice/pointer for nil. An
	// explicit empty array `[]` unmarshals to a non-nil zero-length
	// slice; an omitted key leaves it nil.
	hashesProvided := sel.Hashes != nil
	hasFilter := sel.Filter != nil
	switch {
	case hashesProvided && hasFilter:
		return nil, &ToolError{
			Code:      CodeInvalidArgument,
			Message:   "pass exactly one of hashes or filter, not both",
			Retriable: false,
		}
	case !hashesProvided && !hasFilter:
		return nil, &ToolError{
			Code:      CodeInvalidArgument,
			Message:   "pass either hashes (explicit set, possibly empty) or filter (states/tags); refusing to operate on every download",
			Retriable: false,
		}
	}

	if hashesProvided {
		// May be empty — caller treats as no-op.
		return sel.Hashes, nil
	}

	if terr := validateStates(sel.Filter.States); terr != nil {
		return nil, terr
	}
	if terr := validateTagPatterns(sel.Filter.Tags); terr != nil {
		return nil, terr
	}
	downloads, err := client.GetTorrentsCtx(ctx, qbt.TorrentFilterOptions{})
	if err != nil {
		return nil, errorFromSDK(err)
	}
	filtered, terr := filterDownloads(downloads, sel.Filter.States, sel.Filter.Tags)
	if terr != nil {
		return nil, terr
	}
	out := make([]string, 0, len(filtered))
	for _, t := range filtered {
		out = append(out, t.Hash)
	}
	return out, nil
}

// RemoveDownloadsInput selects downloads to remove from qBittorrent tracking.
type RemoveDownloadsInput struct {
	HashesOrFilter
}

// removeDownloadsHandler removes downloads from qBittorrent's tracking.
// Files on disk are intentionally left intact — deleteFiles is hardcoded
// false so agents cannot reach through and delete arbitrary paths under
// destination aliases. File lifecycle is an operator concern (cron, kura
// trash, manual rm). Audit-logged at WARN since "qbit forgot this
// download" is the kind of event operators investigate when something
// downstream breaks.
func removeDownloadsHandler(client *qbt.Client, logger *slog.Logger) internalHandler[RemoveDownloadsInput, AffectedOutput] {
	return func(ctx context.Context, in RemoveDownloadsInput) (AffectedOutput, *ToolError) {
		hashes, terr := resolveTargets(ctx, client, in.HashesOrFilter)
		if terr != nil {
			return AffectedOutput{AffectedHashes: []string{}}, terr
		}
		if len(hashes) == 0 {
			return AffectedOutput{AffectedHashes: []string{}}, nil
		}
		auditMutation(ctx, logger, slog.LevelWarn, "remove", hashes)
		if err := client.DeleteTorrentsCtx(ctx, hashes, false); err != nil {
			return AffectedOutput{AffectedHashes: []string{}}, errorFromSDK(err)
		}
		return AffectedOutput{AffectedCount: len(hashes), AffectedHashes: hashes}, nil
	}
}
