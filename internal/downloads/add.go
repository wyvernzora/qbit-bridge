package downloads

import (
	"context"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	qbt "github.com/autobrr/go-qbittorrent"

	"github.com/wyvernzora/qbit-bridge/internal/savepath"
)

// --- add_download ---

// AddDownloadInput describes a magnet add request.
type AddDownloadInput struct {
	Magnet      string   `json:"magnet" jsonschema:"magnet URI with xt=urn:btih:<hash> parameter. URLs and torrent-file uploads are not supported in v1."`
	Tags        []string `json:"tags,omitempty" jsonschema:"tags to apply on add; unknown tags are auto-created. tag names must not contain commas (qBittorrent stores tag lists CSV-encoded)."`
	Destination string   `json:"destination,omitempty" jsonschema:"save-destination alias. Accepts '<alias>' for the alias root or '<alias>:<relpath>' to target a subdirectory under the root (relpath must not start with '/' or contain '..'). Reserved name 'unspecified' rejected — output-only sentinel. Empty inherits qBittorrent's account default."`
	Rename      string   `json:"rename,omitempty" jsonschema:"rename download inside qBittorrent on add (display name override)"`
}

// AddDownloadOutput reports whether qBittorrent accepted a magnet add request.
type AddDownloadOutput struct {
	Hash           string `json:"hash"`
	Accepted       bool   `json:"accepted"`
	AlreadyExisted bool   `json:"already_existed"`
}

func addDownloadHandler(client *qbt.Client, resolver *savepath.Resolver, logger *slog.Logger) internalHandler[AddDownloadInput, AddDownloadOutput] {
	return func(ctx context.Context, in AddDownloadInput) (AddDownloadOutput, *ToolError) {
		empty := AddDownloadOutput{}

		hash, terr := parseMagnetHash(in.Magnet)
		if terr != nil {
			return empty, terr
		}
		if terr := validateAddDownloadTags(in.Tags); terr != nil {
			return empty, terr
		}
		savePath, rerr := resolver.Resolve(in.Destination)
		if rerr != nil {
			return empty, &ToolError{Code: CodeInvalidArgument, Message: rerr.Error(), Retriable: false}
		}

		// Idempotent pre-check: qBittorrent's POST /torrents/add returns
		// "Ok." even when the hash is already present, so the response
		// alone can't distinguish "new add" from "re-add of existing
		// download". We probe via /torrents/info?hashes=<h> before the
		// add — if the hash is already known, we skip the upstream call
		// entirely, leaving the live download (tags, destination,
		// progress) untouched. The audit log carries already_existed so
		// agents and operators can tell the noop case apart.
		existing, err := client.GetTorrentsCtx(ctx, qbt.TorrentFilterOptions{Hashes: []string{hash}})
		if err != nil {
			return empty, errorFromSDK(err)
		}
		alreadyExisted := len(existing) > 0

		auditMutation(ctx, logger, slog.LevelInfo, "add", []string{hash},
			slog.String("destination", in.Destination),
			slog.Any("tags", in.Tags),
			slog.Bool("already_existed", alreadyExisted),
		)
		if alreadyExisted {
			return AddDownloadOutput{Hash: hash, Accepted: true, AlreadyExisted: true}, nil
		}

		// Force autoTMM=false on every add. qBittorrent's Automatic Torrent
		// Management auto-routes save_path based on category — if it were
		// left on, the destination alias we just resolved would be silently
		// overridden by the operator's category routing. Pinning false
		// preserves the security boundary the destination alias system
		// exists to enforce.
		opts := map[string]string{"autoTMM": "false"}
		if savePath != "" {
			opts["savepath"] = savePath
		}
		if len(in.Tags) > 0 {
			opts["tags"] = strings.Join(in.Tags, ",")
		}
		if in.Rename != "" {
			opts["rename"] = in.Rename
		}

		if err := client.AddTorrentFromUrlCtx(ctx, in.Magnet, opts); err != nil {
			return empty, errorFromSDK(err)
		}
		return AddDownloadOutput{Hash: hash, Accepted: true, AlreadyExisted: false}, nil
	}
}

// parseMagnetHash extracts and normalizes the btih info-hash from a magnet
// URI. Accepts 40-char hex (returned lowercased) and 32-char base32
// (decoded to 20 bytes and re-encoded as hex). Returns invalid_argument
// for any other shape — missing scheme, no xt=urn:btih:, malformed hash.
func parseMagnetHash(magnet string) (string, *ToolError) {
	if magnet == "" {
		return "", &ToolError{Code: CodeInvalidArgument, Message: "magnet is required", Retriable: false}
	}
	if !strings.HasPrefix(magnet, "magnet:") {
		return "", &ToolError{Code: CodeInvalidArgument, Message: "magnet must start with 'magnet:'", Retriable: false}
	}
	q := strings.TrimPrefix(magnet, "magnet:")
	q = strings.TrimPrefix(q, "?")
	values, err := url.ParseQuery(q)
	if err != nil {
		return "", &ToolError{Code: CodeInvalidArgument, Message: "magnet query string invalid: " + err.Error(), Retriable: false}
	}
	for _, xt := range values["xt"] {
		const prefix = "urn:btih:"
		if !strings.HasPrefix(xt, prefix) {
			continue
		}
		raw := strings.TrimPrefix(xt, prefix)
		if normalized, ok := normalizeBtihHash(raw); ok {
			return normalized, nil
		}
	}
	return "", &ToolError{Code: CodeInvalidArgument, Message: "magnet missing xt=urn:btih:<hash> with a valid 40-hex or 32-base32 hash", Retriable: false}
}

// normalizeBtihHash returns the 40-char lowercase hex form of a btih hash,
// converting from 32-char base32 when needed. Returns ok=false for any
// other length or invalid encoding.
func normalizeBtihHash(h string) (string, bool) {
	switch len(h) {
	case 40:
		lower := strings.ToLower(h)
		for i := 0; i < len(lower); i++ {
			c := lower[i]
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				return "", false
			}
		}
		return lower, true
	case 32:
		raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(h))
		if err != nil || len(raw) != 20 {
			return "", false
		}
		return hex.EncodeToString(raw), true
	default:
		return "", false
	}
}

func validateAddDownloadTags(tags []string) *ToolError {
	for _, t := range tags {
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
