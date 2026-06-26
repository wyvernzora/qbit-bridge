package downloads

import (
	"fmt"
	"path"
	"strings"
)

// validateStates normalizes each entry to lowercase before checking the
// enum, then writes the lowercased value back so downstream filtering
// matches the canonical form. The model commonly emits "Downloading" /
// "DOWNLOADING" / "downloading" interchangeably depending on prior
// context; rejecting non-lowercase variants would be a footgun for no
// reason — the underlying state strings are case-insensitive on the
// wire.
func validateStates(states []NormalizedState) *ToolError {
	for i, s := range states {
		lower := NormalizedState(strings.ToLower(string(s)))
		if !isValidNormalizedState(lower) {
			return &ToolError{
				Code:      CodeInvalidArgument,
				Message:   fmt.Sprintf("unknown state %q; valid: downloading, seeding, paused, stalled, queued, checking, errored, unknown", s),
				Retriable: false,
			}
		}
		states[i] = lower
	}
	return nil
}

func isValidNormalizedState(s NormalizedState) bool {
	switch s {
	case StateDownloading, StateSeeding, StatePaused, StateStalled,
		StateQueued, StateChecking, StateErrored, StateUnknown:
		return true
	}
	return false
}

func validateTagPatterns(patterns []string) *ToolError {
	for _, p := range patterns {
		if _, err := path.Match(p, ""); err != nil {
			return &ToolError{
				Code:      CodeInvalidArgument,
				Message:   fmt.Sprintf("invalid tag pattern %q: %v", p, err),
				Retriable: false,
			}
		}
	}
	return nil
}
