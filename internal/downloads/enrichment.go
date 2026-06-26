package downloads

import (
	"context"

	qbt "github.com/autobrr/go-qbittorrent"
)

// trackerStatusString converts autobrr's integer TrackerStatus enum into a
// readable wire value. Maps to qBittorrent's documented categories;
// unknown codes pass through as "unknown".
func trackerStatusString(s qbt.TrackerStatus) string {
	switch s {
	case qbt.TrackerStatusDisabled:
		return "disabled"
	case qbt.TrackerStatusNotContacted:
		return "not_contacted"
	case qbt.TrackerStatusOK:
		return "working"
	case qbt.TrackerStatusUpdating:
		return "updating"
	case qbt.TrackerStatusNotWorking:
		return "not_working"
	default:
		return "unknown"
	}
}

func projectTracker(t qbt.TorrentTracker) DownloadTracker {
	return DownloadTracker{
		URL:         t.Url,
		Status:      trackerStatusString(t.Status),
		NumPeers:    t.NumPeers,
		NumSeeds:    t.NumSeeds,
		NumLeechers: t.NumLeechers,
		Message:     t.Message,
	}
}

func projectFile(f qbt.TorrentFile) DownloadFile {
	return DownloadFile{
		Name:     f.Name,
		Size:     f.Size,
		Progress: float64(f.Progress),
		Priority: f.Priority,
	}
}

func fetchTrackers(ctx context.Context, client *qbt.Client, hash string, d *Download) *ToolError {
	trackers, err := client.GetTorrentTrackersCtx(ctx, hash)
	if err != nil {
		return errorFromSDK(err)
	}
	d.Trackers = make([]DownloadTracker, 0, len(trackers))
	for _, tr := range trackers {
		d.Trackers = append(d.Trackers, projectTracker(tr))
	}
	return nil
}

func fetchFiles(ctx context.Context, client *qbt.Client, hash string, d *Download) *ToolError {
	files, err := client.GetFilesInformationCtx(ctx, hash)
	if err != nil {
		return errorFromSDK(err)
	}
	if files == nil {
		d.Files = []DownloadFile{}
		return nil
	}
	d.Files = make([]DownloadFile, 0, len(*files))
	for _, f := range *files {
		d.Files = append(d.Files, projectFile(f))
	}
	return nil
}
