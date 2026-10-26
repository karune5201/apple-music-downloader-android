package app

import (
	"amdl/internal/logx"
	"amdl/internal/model"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (r *Runner) isLossySource(ext string, codec string) bool {
	ext = strings.ToLower(ext)
	if ext == ".m4a" && (codec == "AAC" || strings.Contains(codec, "AAC") || strings.Contains(codec, "ATMOS")) {
		return true
	}
	if ext == ".mp3" || ext == ".opus" || ext == ".ogg" {
		return true
	}
	return false
}

// CONVERSION FEATURE: Build ffmpeg arguments for desired target.

func (r *Runner) buildFFmpegArgs(ffmpegPath, inPath, outPath, targetFmt, extraArgs string) ([]string, error) {
	args := []string{"-y", "-i", inPath, "-loglevel", "error", "-map_metadata"}
	if r.Config.Convert.WithMetadata {
		args = append(args, "0")
	} else {
		args = append(args, "-1")
	}
	switch targetFmt {
	case "flac":
		args = append(args, "-c:a", "flac")
		args = append(args, coverArtArgs()...)
	case "mp3":
		// VBR quality 2 ~ high quality
		args = append(args, "-c:a", "libmp3lame", "-qscale:a", "2")
		args = append(args, coverArtArgs()...)
		args = append(args, mp3TagArgs()...)
	case "opus":
		// Ogg Opus cannot carry an attached picture, so drop any video stream
		// explicitly rather than relying on ffmpeg's stream auto-selection.
		args = append(args, "-vn", "-c:a", "libopus", "-b:a", "192k", "-vbr", "on")
	case "wav":
		// WAV has no attached-picture support either; audio only.
		args = append(args, "-vn", "-c:a", "pcm_s16le")
	case "copy":
		// Just container copy (probably pointless for same container)
		args = append(args, "-c", "copy")
	default:
		return nil, fmt.Errorf("unsupported convert-format: %s", targetFmt)
	}
	if extraArgs != "" {
		// naive split; for complex quoting you could enhance
		args = append(args, strings.Fields(extraArgs)...)
	}
	args = append(args, outPath)
	return args, nil
}

// coverArtArgs maps every stream and copies the embedded cover (attached_pic)
// over verbatim, so album art survives the conversion instead of being dropped
// (no -map) or re-encoded (ffmpeg turns a jpg cover into a png on its own).
//
// Only formats whose muxer supports attached pictures may use this: the ogg
// based muxers (opus) reject a picture stream with "Unsupported codec id in
// stream N" and leave an empty file behind.
func coverArtArgs() []string {
	return []string{"-map", "0", "-c:v", "copy", "-disposition:v", "attached_pic"}
}

// mp3TagArgs makes the cover readable for players that are picky about ID3:
// ffmpeg writes ID3v2.4 by default, which Windows Explorer will not read a
// cover from, and leaves the APIC picture type at "Other". The stream comment
// maps to the picture type and the title to the APIC description.
func mp3TagArgs() []string {
	return []string{
		"-id3v2_version", "3",
		"-metadata:s:v", "title=Album cover",
		"-metadata:s:v", "comment=Cover (front)",
	}
}

// CONVERSION FEATURE: Perform conversion if enabled.

func (r *Runner) convertIfNeeded(track *model.Track) {
	if !r.Config.Convert.AfterDownload {
		return
	}
	if r.Config.Convert.Format == "" {
		return
	}
	srcPath := track.SavePath
	if srcPath == "" {
		return
	}
	ext := strings.ToLower(filepath.Ext(srcPath))
	targetFmt := strings.ToLower(r.Config.Convert.Format)

	// Map extension for output
	if targetFmt == "copy" {
		logx.WarnT("app.convert.copy_skipped")
		return
	}

	if r.Config.Convert.SkipIfSourceMatch {
		if ext == "."+targetFmt {
			logx.DetailT("app.convert.skip_already", targetFmt)
			return
		}
	}

	outBase := strings.TrimSuffix(srcPath, ext)
	outPath := outBase + "." + targetFmt

	// Handle lossy -> lossless cases: optionally skip or warn
	if (targetFmt == "flac" || targetFmt == "wav") && r.isLossySource(ext, track.Codec) {
		if r.Config.Convert.SkipLossyToLossless {
			logx.DetailT("app.convert.skip_lossy_to_lossless")
			return
		}
		if r.Config.Convert.WarnLossyToLossless {
			logx.WarnT("app.convert.warn_lossy_to_lossless")
		}
	}

	if _, err := exec.LookPath(r.Config.Convert.FFmpegPath); err != nil {
		logx.WarnT("app.convert.ffmpeg_missing", r.Config.Convert.FFmpegPath)
		return
	}

	args, err := r.buildFFmpegArgs(r.Config.Convert.FFmpegPath, srcPath, outPath, targetFmt, r.Config.Convert.ExtraArgs)
	if err != nil {
		logx.ErrorT("app.convert.config_error", err)
		return
	}

	logx.StepT("app.convert.converting", targetFmt)
	cmd := exec.Command(r.Config.Convert.FFmpegPath, args...)
	var stderr bytes.Buffer
	if r.Config.Convert.CheckBadALAC {
		cmd.Stderr = &stderr
	} else {
		cmd.Stderr = nil
	}
	cmd.Stdout = nil
	start := time.Now()
	if err := cmd.Run(); err != nil {
		logx.ErrorT("app.convert.failed", err)
		// leave original
		return
	}
	if r.Config.Convert.CheckBadALAC && stderr.Len() > 0 {
		logx.WarnT("app.convert.bad_alac_detected")
		if r.Config.Convert.DeleteBadALAC {
			delPath := strings.TrimSuffix(srcPath, "m4a") + targetFmt
			logPath := strings.TrimSuffix(srcPath, "m4a") + "log"
			if err := os.Remove(delPath); err != nil {
				logx.WarnT("app.convert.remove_output_failed", err)
			} else {
				logx.WarnT("app.convert.removed_bad_alac")
				log := stderr
				err = os.WriteFile(logPath, log.Bytes(), 0644)
				if err != nil {
					logx.DetailT("app.convert.logs", log.String())
				} else {
					logx.DetailT("app.convert.logs_saved", logPath)
				}
			}
		}
	} else {
		logx.OKT("app.convert.completed", logx.Since(start), filepath.Base(outPath))

		if !r.Config.Convert.KeepOriginal {
			if err := os.Remove(srcPath); err != nil {
				logx.WarnT("app.convert.remove_original_failed", err)
			} else {
				logx.DetailT("app.convert.original_removed")
			}

		}
		track.SavePath = outPath
		track.SaveName = filepath.Base(outPath)
	}

}
