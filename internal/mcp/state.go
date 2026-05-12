package mcp

import qbt "github.com/autobrr/go-qbittorrent"

// NormalizedState collapses qBittorrent's 21 raw states into the 8 buckets
// agents actually reason about.
type NormalizedState string

const (
	StateDownloading NormalizedState = "downloading"
	StateSeeding     NormalizedState = "seeding"
	StatePaused      NormalizedState = "paused"
	StateStalled     NormalizedState = "stalled"
	StateQueued      NormalizedState = "queued"
	StateChecking    NormalizedState = "checking"
	StateErrored     NormalizedState = "errored"
	StateUnknown     NormalizedState = "unknown"
)

// normalizeState maps a raw qBittorrent state into our enum.
func normalizeState(s qbt.TorrentState) NormalizedState {
	switch s {
	case qbt.TorrentStateDownloading, qbt.TorrentStateMetaDl, qbt.TorrentStateForcedDl, qbt.TorrentStateAllocating:
		return StateDownloading
	case qbt.TorrentStateUploading, qbt.TorrentStateForcedUp:
		return StateSeeding
	case qbt.TorrentStatePausedDl, qbt.TorrentStatePausedUp, qbt.TorrentStateStoppedDl, qbt.TorrentStateStoppedUp:
		return StatePaused
	case qbt.TorrentStateStalledDl, qbt.TorrentStateStalledUp:
		return StateStalled
	case qbt.TorrentStateQueuedDl, qbt.TorrentStateQueuedUp:
		return StateQueued
	case qbt.TorrentStateCheckingDl, qbt.TorrentStateCheckingUp, qbt.TorrentStateCheckingResumeData, qbt.TorrentStateMoving:
		return StateChecking
	case qbt.TorrentStateError, qbt.TorrentStateMissingFiles:
		return StateErrored
	default:
		return StateUnknown
	}
}
