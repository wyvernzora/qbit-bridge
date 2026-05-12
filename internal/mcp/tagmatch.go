package mcp

import (
	"fmt"
	"path"
)

// matchAnyTag reports whether at least one of patterns matches at least one
// of torrentTags. Patterns use stdlib path.Match shell-glob syntax: `*`
// matches any run of characters, `?` matches one, `[abc]` matches a class.
// Patterns without metacharacters degrade to exact equality.
//
// Empty patterns slice returns true (no filter applied).
//
// A malformed pattern returns a non-nil error naming the offending pattern;
// handlers translate that to a *ToolError with CodeInvalidArgument so the
// caller sees which input was bad.
//
// Note: path.Match treats `/` as a separator (`*` will not cross it). This
// is acceptable for the qBittorrent tag namespace which does not customarily
// use slashes; revisit if a real-world tag with `/` shows up.
func matchAnyTag(patterns, torrentTags []string) (bool, error) {
	if len(patterns) == 0 {
		return true, nil
	}
	for _, p := range patterns {
		for _, t := range torrentTags {
			ok, err := path.Match(p, t)
			if err != nil {
				return false, fmt.Errorf("invalid tag pattern %q: %w", p, err)
			}
			if ok {
				return true, nil
			}
		}
	}
	return false, nil
}
