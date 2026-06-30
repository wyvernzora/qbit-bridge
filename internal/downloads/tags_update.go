package downloads

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	qbt "github.com/autobrr/go-qbittorrent"
)

// UpdateDownloadTagsInput selects downloads by explicit hash and patches
// their tag set.
type UpdateDownloadTagsInput struct {
	Hashes []string `json:"hashes" jsonschema:"explicit set of download hashes to retag. required; [] is a no-op."`
	Add    []string `json:"add,omitempty" jsonschema:"literal tag names to add. tag names must not contain commas."`
	Remove []string `json:"remove,omitempty" jsonschema:"literal tag names to remove. tag names must not contain commas."`
}

func updateDownloadTagsHandler(client *qbt.Client, logger *slog.Logger) internalHandler[UpdateDownloadTagsInput, AffectedOutput] {
	return func(ctx context.Context, in UpdateDownloadTagsInput) (AffectedOutput, *ToolError) {
		if terr := validateUpdateDownloadTags(in); terr != nil {
			return AffectedOutput{AffectedHashes: []string{}}, terr
		}
		if len(in.Hashes) == 0 {
			return AffectedOutput{AffectedHashes: []string{}}, nil
		}

		auditMutation(ctx, logger, slog.LevelInfo, "tag", in.Hashes,
			slog.Any("add", in.Add),
			slog.Any("remove", in.Remove),
		)
		if len(in.Add) > 0 {
			if err := client.AddTagsCtx(ctx, in.Hashes, strings.Join(in.Add, ",")); err != nil {
				return AffectedOutput{AffectedHashes: []string{}}, errorFromSDK(err)
			}
		}
		if len(in.Remove) > 0 {
			if err := client.RemoveTagsCtx(ctx, in.Hashes, strings.Join(in.Remove, ",")); err != nil {
				return AffectedOutput{AffectedHashes: []string{}}, errorFromSDK(err)
			}
		}
		return AffectedOutput{AffectedCount: len(in.Hashes), AffectedHashes: in.Hashes}, nil
	}
}

func validateUpdateDownloadTags(in UpdateDownloadTagsInput) *ToolError {
	if in.Hashes == nil {
		return &ToolError{
			Code:      CodeInvalidArgument,
			Message:   "hashes is required (pass [] for no-op)",
			Retriable: false,
		}
	}
	if len(in.Add) == 0 && len(in.Remove) == 0 {
		return &ToolError{
			Code:      CodeInvalidArgument,
			Message:   "pass at least one tag in add or remove",
			Retriable: false,
		}
	}
	for _, tag := range in.Add {
		if terr := validateLiteralDownloadTag(tag); terr != nil {
			return terr
		}
	}
	for _, tag := range in.Remove {
		if terr := validateLiteralDownloadTag(tag); terr != nil {
			return terr
		}
	}
	return nil
}

func validateLiteralDownloadTag(tag string) *ToolError {
	if strings.Contains(tag, ",") {
		return &ToolError{
			Code:      CodeInvalidArgument,
			Message:   fmt.Sprintf("tag %q contains a comma; qBittorrent stores tags CSV-encoded so commas inside a tag would corrupt the list", tag),
			Retriable: false,
		}
	}
	return nil
}
