package downloads

import (
	"context"

	qbt "github.com/autobrr/go-qbittorrent"

	"github.com/wyvernzora/qbit-bridge/internal/savepath"
)

// --- qbit_search_downloads ---

// SearchDownloadsInput filters, sorts, paginates, and projects downloads.
type SearchDownloadsInput struct {
	States        []NormalizedState `json:"states,omitempty" jsonschema:"optional state filter; OR semantics across the array. one of downloading, seeding, paused, stalled, queued, checking, errored, unknown"`
	Tags          []string          `json:"tags,omitempty" jsonschema:"tag-pattern filter; OR semantics. each entry is a shell-style glob (path.Match: *, ?, [abc]); plain strings match exactly. example: ['tvdb:*'] finds every download tagged tvdb:NNNNN."`
	Hashes        []string          `json:"hashes,omitempty" jsonschema:"explicit set of hashes to return. when combined with states/tags, hashes are pre-filtered upstream and states/tags further restrict the result (AND semantics)."`
	Sort          string            `json:"sort,omitempty" jsonschema:"sort order; one of name_asc, name_desc, added_on_asc, added_on_desc (default), size_asc, size_desc, progress_asc, progress_desc, dlspeed_desc, eta_asc, ratio_desc"`
	Limit         int               `json:"limit,omitempty" jsonschema:"max downloads to return; default 50, max 200"`
	Offset        int               `json:"offset,omitempty" jsonschema:"page offset; default 0"`
	IncludeFields []string          `json:"include_fields,omitempty" jsonschema:"opt-in fields. lean defaults to none. valid: save_path, content_path, magnet_uri, completion_on, last_activity, total_uploaded, total_downloaded, total_size, seeds, seeds_incomplete, peers, tracker_count, seeding_time, private, trackers, files. Special value 'all' expands every field-level key except trackers/files. trackers and files trigger one additional upstream call per result; response size scales with hashes * per-torrent fan-out."`
}

// SearchDownloadsOutput is the paginated download search response.
type SearchDownloadsOutput struct {
	Count     int        `json:"count"`
	HasMore   bool       `json:"has_more"`
	Downloads []Download `json:"downloads"`
}

const (
	defaultSearchLimit = 50
	maxSearchLimit     = 200
)

// searchDownloadsRequest is the validated, ready-to-execute form of a
// SearchDownloadsInput. prepareSearchDownloads produces it after every
// validation rule, keeping the handler body thin.
type searchDownloadsRequest struct {
	opts       qbt.TorrentFilterOptions
	includeSet map[string]bool
	limit      int
	offset     int
}

func searchDownloadsHandler(client *qbt.Client, resolver *savepath.Resolver) internalHandler[SearchDownloadsInput, SearchDownloadsOutput] {
	return func(ctx context.Context, in SearchDownloadsInput) (SearchDownloadsOutput, *ToolError) {
		empty := SearchDownloadsOutput{Downloads: []Download{}}

		req, terr := prepareSearchDownloads(in)
		if terr != nil {
			return empty, terr
		}

		downloads, err := client.GetTorrentsCtx(ctx, req.opts)
		if err != nil {
			return empty, errorFromSDK(err)
		}

		filtered, terr := filterDownloads(downloads, in.States, in.Tags)
		if terr != nil {
			return empty, terr
		}

		page, hasMore := paginateDownloads(filtered, req.offset, req.limit)
		out := SearchDownloadsOutput{
			Count:     len(page),
			HasMore:   hasMore,
			Downloads: make([]Download, 0, len(page)),
		}
		for _, t := range page {
			d := projectDownload(t, req.includeSet, resolver)
			if req.includeSet["trackers"] {
				if terr := fetchTrackers(ctx, client, t.Hash, &d); terr != nil {
					return empty, terr
				}
			}
			if req.includeSet["files"] {
				if terr := fetchFiles(ctx, client, t.Hash, &d); terr != nil {
					return empty, terr
				}
			}
			out.Downloads = append(out.Downloads, d)
		}
		return out, nil
	}
}

// prepareSearchDownloads validates input and assembles the upstream filter
// options plus the resolved include-fields set and clamped limit/offset.
func prepareSearchDownloads(in SearchDownloadsInput) (searchDownloadsRequest, *ToolError) {
	limit, terr := normalizeSearchLimit(in.Limit)
	if terr != nil {
		return searchDownloadsRequest{}, terr
	}
	offset, terr := normalizeSearchOffset(in.Offset)
	if terr != nil {
		return searchDownloadsRequest{}, terr
	}
	sortField, reverse, terr := resolveSort(in.Sort)
	if terr != nil {
		return searchDownloadsRequest{}, terr
	}
	if terr := validateStates(in.States); terr != nil {
		return searchDownloadsRequest{}, terr
	}
	if terr := validateTagPatterns(in.Tags); terr != nil {
		return searchDownloadsRequest{}, terr
	}
	includeSet, terr := resolveIncludeFields(in.IncludeFields)
	if terr != nil {
		return searchDownloadsRequest{}, terr
	}

	opts := qbt.TorrentFilterOptions{Sort: sortField, Reverse: reverse}
	if len(in.Hashes) > 0 {
		opts.Hashes = in.Hashes
	}
	return searchDownloadsRequest{opts: opts, includeSet: includeSet, limit: limit, offset: offset}, nil
}

// filterDownloads applies the state-set and tag-glob filters that the qbit
// API cannot express natively. Tag patterns are assumed pre-validated by
// validateTagPatterns; a re-failure here is treated as an internal error.
func filterDownloads(downloads []qbt.Torrent, states []NormalizedState, tagPatterns []string) ([]qbt.Torrent, *ToolError) {
	stateSet := make(map[NormalizedState]bool, len(states))
	for _, s := range states {
		stateSet[s] = true
	}
	out := make([]qbt.Torrent, 0, len(downloads))
	for _, t := range downloads {
		if len(stateSet) > 0 && !stateSet[normalizeState(t.State)] {
			continue
		}
		if len(tagPatterns) > 0 {
			ok, err := matchAnyTag(tagPatterns, splitTags(t.Tags))
			if err != nil {
				return nil, &ToolError{Code: CodeInternal, Message: err.Error(), Retriable: false}
			}
			if !ok {
				continue
			}
		}
		out = append(out, t)
	}
	return out, nil
}

// paginateDownloads slices the filtered set per offset+limit and reports
// whether more downloads exist past the returned page. Out-of-range offsets
// yield an empty page (not an error).
func paginateDownloads(filtered []qbt.Torrent, offset, limit int) ([]qbt.Torrent, bool) {
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

func normalizeSearchLimit(in int) (int, *ToolError) {
	if in < 0 {
		return 0, &ToolError{Code: CodeInvalidArgument, Message: "limit must be >= 0", Retriable: false}
	}
	if in == 0 {
		return defaultSearchLimit, nil
	}
	if in > maxSearchLimit {
		return maxSearchLimit, nil
	}
	return in, nil
}

func normalizeSearchOffset(in int) (int, *ToolError) {
	if in < 0 {
		return 0, &ToolError{Code: CodeInvalidArgument, Message: "offset must be >= 0", Retriable: false}
	}
	return in, nil
}
