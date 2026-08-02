package audio

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ProbeDuration returns the duration of an audio file in seconds.
func ProbeDuration(ctx context.Context, audioFile string) (float64, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		audioFile,
	)
	out, err := cmd.Output()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, ctxErr
		}
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

// ExtractChunkWAV extracts a chunk from an audio file to a WAV file.
func ExtractChunkWAV(ctx context.Context, audioFile, chunkFile string, start, duration float64) error {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-ss", fmt.Sprintf("%.3f", start),
		"-t", fmt.Sprintf("%.3f", duration),
		"-i", audioFile,
		"-vn",
		"-ar", "16000",
		"-ac", "1",
		"-acodec", "pcm_s16le",
		"-y",
		chunkFile,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("ffmpeg: %w: %s", err, out)
	}
	return nil
}
