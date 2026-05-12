package mcp

import "testing"

func TestMatchAnyTag(t *testing.T) {
	cases := []struct {
		name     string
		patterns []string
		tags     []string
		want     bool
	}{
		{
			name:     "empty patterns matches everything",
			patterns: nil,
			tags:     []string{"anything"},
			want:     true,
		},
		{
			name:     "exact literal hit",
			patterns: []string{"tvdb:12345"},
			tags:     []string{"weekly", "tvdb:12345"},
			want:     true,
		},
		{
			name:     "exact literal miss",
			patterns: []string{"tvdb:99999"},
			tags:     []string{"weekly", "tvdb:12345"},
			want:     false,
		},
		{
			name:     "prefix glob",
			patterns: []string{"tvdb:*"},
			tags:     []string{"weekly", "tvdb:12345"},
			want:     true,
		},
		{
			name:     "suffix glob",
			patterns: []string{"*-anime"},
			tags:     []string{"weekly-anime"},
			want:     true,
		},
		{
			name:     "bidirectional glob",
			patterns: []string{"*tvdb*"},
			tags:     []string{"kura-tvdb-job"},
			want:     true,
		},
		{
			name:     "OR across patterns; first hits",
			patterns: []string{"tvdb:*", "kura:*"},
			tags:     []string{"tvdb:12345"},
			want:     true,
		},
		{
			name:     "OR across patterns; second hits",
			patterns: []string{"tvdb:*", "kura:*"},
			tags:     []string{"kura:reconcile"},
			want:     true,
		},
		{
			name:     "OR across patterns; neither hits",
			patterns: []string{"tvdb:*", "kura:*"},
			tags:     []string{"weekly", "complete"},
			want:     false,
		},
		{
			name:     "character class",
			patterns: []string{"tvdb:[0-9]*"},
			tags:     []string{"tvdb:12345"},
			want:     true,
		},
		{
			name:     "download has no tags",
			patterns: []string{"tvdb:*"},
			tags:     []string{},
			want:     false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := matchAnyTag(tc.patterns, tc.tags)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("matchAnyTag(%v, %v) = %v, want %v", tc.patterns, tc.tags, got, tc.want)
			}
		})
	}
}

func TestMatchAnyTag_RejectsMalformedPattern(t *testing.T) {
	// Unclosed character class is path.Match's classic ErrBadPattern case.
	_, err := matchAnyTag([]string{"tvdb:[unclosed"}, []string{"tvdb:1"})
	if err == nil {
		t.Fatal("expected error for malformed pattern")
	}
	if !contains(err.Error(), "tvdb:[unclosed") {
		t.Errorf("error %q should name the offending pattern", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
