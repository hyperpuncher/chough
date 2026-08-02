package audio

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestFFmpegCommandsHonorCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := ProbeDuration(ctx, "audio.mp3"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ProbeDuration() error = %v, want context.Canceled", err)
	}
	output := filepath.Join(t.TempDir(), "chunk.wav")
	if err := ExtractChunkWAV(ctx, "audio.mp3", output, 0, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("ExtractChunkWAV() error = %v, want context.Canceled", err)
	}
}
