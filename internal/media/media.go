package media

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func DownloadOrCopyVideo(ctx context.Context, sourceType, sourceURL, sourcePath, dstPath, ytdlpBin string) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}

	switch sourceType {
	case "url":
		if sourceURL == "" {
			return fmt.Errorf("source_url is required for source_type=url")
		}
		// Force a merged MP4 output at the exact destination path.
		cmd := exec.CommandContext(ctx, ytdlpBin,
			"--merge-output-format", "mp4",
			"-o", dstPath,
			sourceURL,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("yt-dlp failed: %w: %s", err, string(out))
		}
		return nil
	case "path":
		if sourcePath == "" {
			return fmt.Errorf("source_path is required for source_type=path")
		}
		return copyFile(sourcePath, dstPath)
	default:
		return fmt.Errorf("unsupported source_type: %s", sourceType)
	}
}

func ExtractAudio(ctx context.Context, ffmpegBin, videoPath, audioPath string) error {
	if err := os.MkdirAll(filepath.Dir(audioPath), 0o755); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, ffmpegBin,
		"-y",
		"-i", videoPath,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		audioPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w: %s", err, string(out))
	}
	return nil
}

func copyFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return dst.Sync()
}
