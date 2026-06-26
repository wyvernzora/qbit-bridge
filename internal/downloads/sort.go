package downloads

import (
	"fmt"
	"sort"
	"strings"
)

// sortSpec maps a public sort enum value to qBittorrent's native sort field
// plus the Reverse flag the SDK passes upstream.
type sortSpec struct {
	field   string
	reverse bool
}

var searchDownloadsSortMap = map[string]sortSpec{
	"name_asc":      {"name", false},
	"name_desc":     {"name", true},
	"added_on_asc":  {"added_on", false},
	"added_on_desc": {"added_on", true},
	"size_asc":      {"size", false},
	"size_desc":     {"size", true},
	"progress_asc":  {"progress", false},
	"progress_desc": {"progress", true},
	"dlspeed_desc":  {"dlspeed", true},
	"eta_asc":       {"eta", false},
	"ratio_desc":    {"ratio", true},
}

func resolveSort(s string) (field string, reverse bool, terr *ToolError) {
	if s == "" {
		s = "added_on_desc"
	}
	spec, ok := searchDownloadsSortMap[s]
	if !ok {
		return "", false, &ToolError{
			Code:      CodeInvalidArgument,
			Message:   fmt.Sprintf("unknown sort %q; valid: %s", s, validSortNames()),
			Retriable: false,
		}
	}
	return spec.field, spec.reverse, nil
}

func validSortNames() string {
	out := make([]string, 0, len(searchDownloadsSortMap))
	for k := range searchDownloadsSortMap {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}
