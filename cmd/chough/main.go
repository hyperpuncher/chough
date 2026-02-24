package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hyperpuncher/chough/internal/asr"
)

// ANSI color codes
const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	white   = "\033[37m"
)

func main() {
	// Define flags
	chunkSize := flag.Int("c", 30, "chunk size in seconds")
	flag.IntVar(chunkSize, "chunk-size", 30, "chunk size in seconds")

	// Pretty usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s🐦‍⬛ %schough%s\n\n", bold, magenta, reset)
		fmt.Fprintf(os.Stderr, "%sUsage:%s chough [flags] <audio-file>\n\n", bold, reset)
		fmt.Fprintf(os.Stderr, "%sFlags:%s\n", bold, reset)
		fmt.Fprintf(os.Stderr, "  %s-c, --chunk-size%s %sint%s    chunk size in seconds %s(default: 30)%s\n",
			cyan, reset, yellow, reset, dim, reset)
		fmt.Fprintf(os.Stderr, "\n%sExamples:%s\n", bold, reset)
		fmt.Fprintf(os.Stderr, "  %s$%s chough audio.mp3 %s# default 30s chunks%s\n",
			green, reset, dim, reset)
		fmt.Fprintf(os.Stderr, "  %s$%s chough -c 60 podcast.mp3 %s# 60s chunks, faster%s\n",
			green, reset, dim, reset)
		fmt.Fprintf(os.Stderr, "  %s$%s chough --chunk-size 15 talk.mp3 %s# 15s chunks, less memory%s\n",
			green, reset, dim, reset)
		fmt.Fprintf(os.Stderr, "\n%sEnvironment:%s\n", bold, reset)
		fmt.Fprintf(os.Stderr, "  %sCHOUGH_MODEL%s    path to model directory %s(required)%s\n",
			cyan, reset, dim, reset)
	}

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	audioFile := flag.Arg(0)
	chunkSecs := *chunkSize

	// Load model once
	fmt.Fprintln(os.Stderr, "Loading model...")
	config := asr.Config{
		ModelPath:  os.Getenv("CHOUGH_MODEL"),
		NumThreads: 4,
		SampleRate: 16000,
		FeatureDim: 80,
		Provider:   "cpu",
	}

	recognizer, err := asr.NewRecognizer(&config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load model: %v\n", err)
		os.Exit(1)
	}
	defer recognizer.Close()
	fmt.Fprintln(os.Stderr, "Model loaded!")

	// Get audio duration
	duration, err := getDuration(audioFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get duration: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Audio: %.1fs, chunks: %ds\n", duration, chunkSecs)

	// Process chunks
	startTime := time.Now()
	var fullText strings.Builder
	chunkCount := int(duration/float64(chunkSecs)) + 1

	for i := 0; i < chunkCount; i++ {
		start := float64(i * chunkSecs)
		end := start + float64(chunkSecs)
		if end > duration {
			end = duration
		}

		if end <= start+0.5 { // Skip chunks smaller than 0.5s
			break
		}

		fmt.Fprintf(os.Stderr, "Chunk %d/%d (%.1fs-%.1fs)... ", i+1, chunkCount, start, end)

		chunkText, err := transcribeChunk(recognizer, audioFile, start, end-start)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERR: %v\n", err)
			continue
		}

		fmt.Fprintf(os.Stderr, "OK (%d chars)\n", len(chunkText))

		if fullText.Len() > 0 {
			fullText.WriteString(" ")
		}
		fullText.WriteString(chunkText)
	}

	elapsed := time.Since(startTime)
	fmt.Fprintf(os.Stderr, "\nDone in %v (%.1fx realtime)\n", elapsed, duration/elapsed.Seconds())
	fmt.Println(fullText.String())
}

func getDuration(audioFile string) (float64, error) {
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

func transcribeChunk(recognizer *asr.Recognizer, audioFile string, start, duration float64) (string, error) {
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "chough-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	chunkFile := filepath.Join(tmpDir, "chunk.wav")

	// Extract chunk with ffmpeg
	cmd := exec.Command("ffmpeg",
		"-ss", fmt.Sprintf("%.3f", start),
		"-t", fmt.Sprintf("%.3f", duration),
		"-i", audioFile,
		"-ar", "16000",
		"-ac", "1",
		"-acodec", "pcm_s16le",
		"-y",
		chunkFile,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg: %s", out)
	}

	// Transcribe with loaded model
	result, err := recognizer.Transcribe(chunkFile)
	if err != nil {
		return "", err
	}

	return result.Text, nil
}
