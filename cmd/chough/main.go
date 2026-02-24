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

func main() {
	// Define flags
	chunkSize := flag.Int("c", 30, "chunk size in seconds")
	flag.IntVar(chunkSize, "chunk-size", 30, "chunk size in seconds")

	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: chough [flags] <audio-file>\n\n")
		fmt.Fprintf(os.Stderr, "A fast, memory-efficient ASR CLI using sherpa-onnx\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  chough audio.mp3              # default 30s chunks\n")
		fmt.Fprintf(os.Stderr, "  chough -c 60 audio.mp3        # 60s chunks\n")
		fmt.Fprintf(os.Stderr, "  chough --chunk-size 15 audio.mp3  # 15s chunks\n")
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
