package mcp

// Shared MCP I/O shapes referenced by multiple tool definitions.

// FilterCriteria selects downloads by state / tag. Used inside
// HashesOrFilter for bulk operations whose callers want filter-based
// selection instead of explicit hashes.
type FilterCriteria struct {
	States []NormalizedState `json:"states,omitempty" jsonschema:"optional download state filter; OR semantics. one of downloading, seeding, paused, stalled, queued, checking, errored, unknown"`
	Tags   []string          `json:"tags,omitempty" jsonschema:"tag-pattern filter; OR semantics across the list. each entry is a shell-style glob (path.Match syntax): '*' matches any run of chars, '?' matches one, '[abc]' matches a class. plain strings without metacharacters match exact tag names. example: ['tvdb:*'] finds every download tagged tvdb:12345, tvdb:67890, …"`
}

// AffectedOutput is the response shape for bulk operations
// (pause/resume/delete/update).
type AffectedOutput struct {
	AffectedCount  int      `json:"affected_count"`
	AffectedHashes []string `json:"affected_hashes"`
}

// OkOutput is the response shape for tools whose only result is success/failure.
type OkOutput struct {
	Ok bool `json:"ok"`
}

// Download is the wire shape returned to MCP clients. Fields tagged
// `omitempty` in the lean default projection are populated only when the
// caller passes the relevant `include_fields` / `include` option on
// list_downloads / get_download.
type Download struct {
	Hash               string          `json:"hash"`
	Name               string          `json:"name"`
	State              NormalizedState `json:"state"`
	Progress           float64         `json:"progress"`
	SizeBytes          int64           `json:"size_bytes"`
	DlspeedBytesPerSec int64           `json:"dlspeed_bytes_per_sec"`
	UpspeedBytesPerSec int64           `json:"upspeed_bytes_per_sec"`
	EtaSeconds         int64           `json:"eta_seconds"`
	Ratio              float64         `json:"ratio"`
	Tags               []string        `json:"tags"`
	AddedOn            int64           `json:"added_on"`

	// Opt-in via list_downloads.include_fields or get_download default-rich set.
	SavePath           string  `json:"save_path,omitempty"`
	ContentPath        string  `json:"content_path,omitempty"`
	DownloadPath       string  `json:"download_path,omitempty"`
	MagnetURI          string  `json:"magnet_uri,omitempty"`
	CompletionOn       int64   `json:"completion_on,omitempty"`
	LastActivity       int64   `json:"last_activity,omitempty"`
	TotalUploaded      int64   `json:"total_uploaded,omitempty"`
	TotalDownloaded    int64   `json:"total_downloaded,omitempty"`
	TotalSize          int64   `json:"total_size,omitempty"`
	SeedsComplete      int64   `json:"seeds_complete,omitempty"`
	SeedsIncomplete    int64   `json:"seeds_incomplete,omitempty"`
	PeersConnected     int64   `json:"peers_connected,omitempty"`
	TrackerCount       int64   `json:"tracker_count,omitempty"`
	AutoTMM            *bool   `json:"auto_tmm,omitempty"`
	Sequential         *bool   `json:"sequential,omitempty"`
	ForceStart         *bool   `json:"force_start,omitempty"`
	SuperSeeding       *bool   `json:"super_seeding,omitempty"`
	FirstLastPiecePrio *bool   `json:"first_last_piece_prio,omitempty"`
	RatioLimit         float64 `json:"ratio_limit,omitempty"`
	SeedingTime        int64   `json:"seeding_time,omitempty"`
	SeedingTimeLimit   int64   `json:"seeding_time_limit,omitempty"`
	Private            *bool   `json:"private,omitempty"`

	// Opt-in via get_download.include = ["trackers", "files"].
	Trackers []DownloadTracker `json:"trackers,omitempty"`
	Files    []DownloadFile    `json:"files,omitempty"`
}

// DownloadTracker is one entry in Download.Trackers.
type DownloadTracker struct {
	URL         string `json:"url"`
	Status      string `json:"status"`
	NumPeers    int    `json:"num_peers"`
	NumSeeds    int    `json:"num_seeds"`
	NumLeechers int    `json:"num_leechers"`
	Message     string `json:"message,omitempty"`
}

// DownloadFile is one entry in Download.Files.
type DownloadFile struct {
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Progress float64 `json:"progress"`
	Priority int     `json:"priority"`
}
