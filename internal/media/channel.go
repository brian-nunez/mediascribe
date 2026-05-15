package media

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type ChannelVideo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

func ListChannelVideos(ctx context.Context, ytdlpBin, channelURL string, limit int) ([]ChannelVideo, error) {
	url := strings.TrimSpace(channelURL)
	if url == "" {
		return nil, fmt.Errorf("channel URL is required")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	var out []byte
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		cmd := exec.CommandContext(ctx, ytdlpBin,
			"--flat-playlist",
			"--playlist-end", fmt.Sprintf("%d", limit),
			"--print", "%(id)s\t%(title)s\t%(webpage_url)s",
			url,
		)
		out, err = cmd.CombinedOutput()
		if err == nil {
			break
		}
		if attempt < 3 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("yt-dlp list failed after retries: %w: %s", err, string(out))
	}

	lines := strings.Split(string(out), "\n")
	items := make([]ChannelVideo, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		id := strings.TrimSpace(parts[0])
		title := strings.TrimSpace(parts[1])
		videoURL := ""
		if len(parts) == 3 {
			videoURL = strings.TrimSpace(parts[2])
		}
		if videoURL == "" && id != "" {
			videoURL = "https://www.youtube.com/watch?v=" + id
		}
		if title == "" {
			title = id
		}
		if videoURL == "" {
			continue
		}
		items = append(items, ChannelVideo{ID: id, Title: title, URL: videoURL})
	}
	return items, nil
}
