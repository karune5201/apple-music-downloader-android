package app

import (
	"amdl/internal/logx"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (r *Runner) saveAnimatedArtwork(dir string, squareVideoURL string, tallVideoURL string) {
	if squareVideoURL != "" {
		if err := r.downloadAnimatedArtwork(dir, "square", squareVideoURL); err != nil {
			logx.WarnT("app.artwork.motion_square_missing", err)
		}
		if r.Config.Metadata.Artwork.EmbyAnimated {
			cmd := exec.Command("ffmpeg", "-i", filepath.Join(dir, "square_animated_artwork.mp4"), "-vf", "scale=440:-1", "-r", "24", "-f", "gif", filepath.Join(dir, "folder.jpg"))
			if err := cmd.Run(); err != nil {
				logx.WarnT("app.artwork.gif_failed", err)
			}
		}
	}
	if tallVideoURL != "" {
		if err := r.downloadAnimatedArtwork(dir, "tall", tallVideoURL); err != nil {
			logx.WarnT("app.artwork.motion_tall_missing", err)
		}
	}
}

func (r *Runner) downloadAnimatedArtwork(dir string, orientation string, videoURL string) error {
	videoURL, err := r.extractVideo(videoURL)
	if err != nil {
		return err
	}
	output := filepath.Join(dir, orientation+"_animated_artwork.mp4")
	exists, err := fileExists(output)
	if err != nil {
		return fmt.Errorf("check existing artwork: %w", err)
	}
	if exists {
		logx.OKT("app.artwork.already_exists", orientation)
		return nil
	}
	logx.StepT("app.artwork.downloading", orientation)
	cmd := exec.Command("ffmpeg", "-loglevel", "quiet", "-y", "-i", videoURL, "-c", "copy", output)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("download animated artwork: %w", err)
	}
	logx.OKT("app.artwork.downloaded", orientation)
	return nil
}

func sanitizeFolderName(name string) string {
	if name == "" {
		return ""
	}
	if name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	return forbiddenNames.ReplaceAllString(name, "_")
}

func (r *Runner) prepareCollectionFolder(base string, displayName string) (string, error) {
	if strings.HasSuffix(displayName, ".") {
		displayName = displayName[:len(displayName)-1]
	}
	displayName = strings.TrimSpace(displayName)
	folderPath := filepath.Join(base, sanitizeFolderName(displayName))
	if err := createDirectory(folderPath); err != nil {
		return "", err
	}
	return folderPath, nil
}

func (r *Runner) saveM3UPlaylist(dir string, name string, startIdx int) {
	if !r.Flags.SaveM3U8 {
		return
	}
	if len(r.State.AddedTracks) > startIdx {
		if err := r.writeM3UPlaylist(dir, name, r.State.AddedTracks[startIdx:]); err != nil {
			logx.ErrorT("app.artwork.m3u8_write_failed", err)
		} else {
			logx.OKT("app.artwork.m3u8_saved")
		}
	}
}

func createDirectory(path string) error {
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return fmt.Errorf("create directory %s: %w", path, err)
	}
	return nil
}
