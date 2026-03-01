package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hyperpuncher/chough/internal/asr"
)

func probeDuration(audioFile string) (float64, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		audioFile,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

func extractChunkWAV(audioFile, chunkFile string, start, duration float64) error {
	cmd := exec.Command("ffmpeg",
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
		return fmt.Errorf("ffmpeg: %s", out)
	}
	return nil
}

func transcribeChunk(recognizer *asr.Recognizer, audioFile string, start, duration float64) (*asr.Result, error) {
	tmpDir, err := os.MkdirTemp("", "chough-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	chunkFile := filepath.Join(tmpDir, "chunk.wav")
	if err := extractChunkWAV(audioFile, chunkFile, start, duration); err != nil {
		return nil, err
	}

	return recognizer.Transcribe(chunkFile)
}
