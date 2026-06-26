package downloads

import (
	"fmt"
	"sort"
	"strings"

	qbt "github.com/autobrr/go-qbittorrent"

	"github.com/wyvernzora/qbit-bridge/internal/savepath"
)

const qbtETAUnknown = 8640000 // qBittorrent's "100 days" sentinel for unknown ETA

// downloadFieldSetter writes one opt-in field on the wire Download from the
// corresponding upstream field. Each include_fields value maps to exactly
// one setter; validation rejects unknown keys before this map is consulted.
type downloadFieldSetter func(out *Download, t qbt.Torrent)

// downloadFieldSetters is the single source of truth for opt-in field
// projection. trackers/files have no setter — they are populated by
// per-hash upstream calls in the handler itself, gated on single-hash
// selection.
var downloadFieldSetters = map[string]downloadFieldSetter{
	"save_path":        func(out *Download, t qbt.Torrent) { out.SavePath = t.SavePath },
	"content_path":     func(out *Download, t qbt.Torrent) { out.ContentPath = t.ContentPath },
	"magnet_uri":       func(out *Download, t qbt.Torrent) { out.MagnetURI = t.MagnetURI },
	"completion_on":    func(out *Download, t qbt.Torrent) { out.CompletionOn = t.CompletionOn },
	"last_activity":    func(out *Download, t qbt.Torrent) { out.LastActivity = t.LastActivity },
	"total_uploaded":   func(out *Download, t qbt.Torrent) { out.TotalUploaded = t.Uploaded },
	"total_downloaded": func(out *Download, t qbt.Torrent) { out.TotalDownloaded = t.Downloaded },
	"total_size":       func(out *Download, t qbt.Torrent) { out.TotalSize = t.TotalSize },
	"seeds":            func(out *Download, t qbt.Torrent) { out.SeedsComplete = t.NumComplete },
	"seeds_incomplete": func(out *Download, t qbt.Torrent) { out.SeedsIncomplete = t.NumIncomplete },
	"peers":            func(out *Download, t qbt.Torrent) { out.PeersConnected = t.NumSeeds + t.NumLeechs },
	"tracker_count":    func(out *Download, t qbt.Torrent) { out.TrackerCount = t.TrackersCount },
	"seeding_time":     func(out *Download, t qbt.Torrent) { out.SeedingTime = t.SeedingTime },
	"private":          func(out *Download, t qbt.Torrent) { v := t.Private; out.Private = &v },
}

// validIncludeFields holds every accepted include_fields value, including
// the per-hash enrichments (trackers, files) that have no field-setter.
// resolveIncludeFields consults this rather than downloadFieldSetters so
// trackers/files validate as known.
var validIncludeFields = func() map[string]bool {
	out := map[string]bool{"trackers": true, "files": true}
	for k := range downloadFieldSetters {
		out[k] = true
	}
	return out
}()

// resolveIncludeFields validates and expands the include_fields list. The
// special value "all" expands to every field-level key (every
// downloadFieldSetters entry) but NOT trackers/files, which always require
// explicit opt-in and trigger the single-hash constraint.
func resolveIncludeFields(in []string) (map[string]bool, *ToolError) {
	out := make(map[string]bool, len(in))
	for _, f := range in {
		if f == "all" {
			for k := range downloadFieldSetters {
				out[k] = true
			}
			continue
		}
		if !validIncludeFields[f] {
			return nil, &ToolError{
				Code:      CodeInvalidArgument,
				Message:   fmt.Sprintf("unknown include_fields value %q; valid: %s, all", f, validIncludeFieldNames()),
				Retriable: false,
			}
		}
		out[f] = true
	}
	return out, nil
}

func validIncludeFieldNames() string {
	out := make([]string, 0, len(validIncludeFields))
	for k := range validIncludeFields {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// splitTags parses qBittorrent's comma-separated Tags string into a slice.
// Whitespace around each tag is trimmed; empty entries are dropped. An
// empty input yields a non-nil empty slice so JSON output is `[]` rather
// than `null`.
func splitTags(csv string) []string {
	out := []string{}
	if csv == "" {
		return out
	}
	for _, p := range strings.Split(csv, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// normalizeETA collapses qBittorrent's 8640000-second "unknown" sentinel
// to -1 so agents have a single value to branch on.
func normalizeETA(eta int64) int64 {
	if eta == qbtETAUnknown {
		return -1
	}
	return eta
}

// projectDownload maps an autobrr qbt.Torrent into the lean MCP wire shape,
// filling opt-in fields per the include set. Trackers and Files are NOT
// populated here — the handler fetches them per-hash when requested.
//
// Path-shaped fields (save_path, content_path, download_path) are
// post-processed through resolver.NameForPathPrefixed when they're
// part of the include set: an absolute path that lives under a
// configured alias root is rewritten to the "<alias>:<relpath>" form
// (or just "<alias>" when the path equals the root exactly). Paths
// outside every configured alias are echoed raw.
func projectDownload(t qbt.Torrent, include map[string]bool, resolver *savepath.Resolver) Download {
	out := Download{
		Hash:               t.Hash,
		Name:               t.Name,
		State:              normalizeState(t.State),
		Progress:           t.Progress,
		SizeBytes:          t.Size,
		DlspeedBytesPerSec: t.DlSpeed,
		UpspeedBytesPerSec: t.UpSpeed,
		EtaSeconds:         normalizeETA(t.ETA),
		Ratio:              t.Ratio,
		Tags:               splitTags(t.Tags),
		AddedOn:            t.AddedOn,
	}
	for key := range include {
		if setter, ok := downloadFieldSetters[key]; ok {
			setter(&out, t)
		}
	}
	if include["save_path"] {
		out.SavePath = prefixed(resolver, out.SavePath)
	}
	if include["content_path"] {
		out.ContentPath = prefixed(resolver, out.ContentPath)
	}
	return out
}

// prefixed rewrites a qBittorrent absolute path into the wire form
// used by every tool output projection (see
// savepath.Resolver.NameForPathPrefixed). Empty input stays empty;
// every non-empty input becomes either "<alias>", "<alias>:<relpath>",
// or "unspecified:<rest>".
func prefixed(resolver *savepath.Resolver, path string) string {
	if path == "" {
		return ""
	}
	return resolver.NameForPathPrefixed(path)
}
