package main

import (
	"encoding/json"
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

// ChunkResult holds transcription result for a single chunk
type ChunkResult struct {
	Index      int
	StartTime  float64
	EndTime    float64
	Text       string
	Timestamps []float32
	Tokens     []string
}

func main() {
	// Define flags
	chunkSize := flag.Int("c", 30, "chunk size in seconds")
	flag.IntVar(chunkSize, "chunk-size", 30, "chunk size in seconds")
	format := flag.String("f", "text", "output format (text, json, vtt)")
	flag.StringVar(format, "format", "text", "output format (text, json, vtt)")

	// Pretty usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s🐦‍⬛ %schough%s\n\n", bold, magenta, reset)
		fmt.Fprintf(os.Stderr, "%sUsage:%s chough [flags] <audio-file>\n\n", bold, reset)
		fmt.Fprintf(os.Stderr, "%sFlags:%s\n", bold, reset)
		fmt.Fprintf(os.Stderr, "  %s-c, --chunk-size%s %sint%s    chunk size in seconds %s(default: 30)%s\n",
			cyan, reset, yellow, reset, dim, reset)
		fmt.Fprintf(os.Stderr, "  %s-f, --format%s %sstring%s    output format: text, json, vtt %s(default: text)%s\n",
			cyan, reset, yellow, reset, dim, reset)
		fmt.Fprintf(os.Stderr, "\n%sExamples:%s\n", bold, reset)
		fmt.Fprintf(os.Stderr, "  %s$%s chough audio.mp3 %s# plain text output%s\n",
			green, reset, dim, reset)
		fmt.Fprintf(os.Stderr, "  %s$%s chough -f vtt audio.mp3 %s# WebVTT subtitles%s\n",
			green, reset, dim, reset)
		fmt.Fprintf(os.Stderr, "  %s$%s chough --format json talk.mp3 %s# JSON with metadata%s\n",
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
	outputFormat := strings.ToLower(*format)

	// Validate format
	if outputFormat != "text" && outputFormat != "json" && outputFormat != "vtt" {
		fmt.Fprintf(os.Stderr, "Error: unknown format %q (valid: text, json, vtt)\n", outputFormat)
		os.Exit(1)
	}

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

	fmt.Fprintf(os.Stderr, "Audio: %.1fs, chunks: %ds, format: %s\n", duration, chunkSecs, outputFormat)

	// Process chunks
	startTime := time.Now()
	chunkCount := int(duration/float64(chunkSecs)) + 1
	var results []ChunkResult

	for i := 0; i < chunkCount; i++ {
		chunkStart := float64(i * chunkSecs)
		chunkEnd := chunkStart + float64(chunkSecs)
		if chunkEnd > duration {
			chunkEnd = duration
		}

		if chunkEnd <= chunkStart+0.5 { // Skip chunks smaller than 0.5s
			break
		}

		fmt.Fprintf(os.Stderr, "Chunk %d/%d (%.1fs-%.1fs)... ", i+1, chunkCount, chunkStart, chunkEnd)

		result, err := transcribeChunk(recognizer, audioFile, chunkStart, chunkEnd-chunkStart)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERR: %v\n", err)
			continue
		}

		// Store result with timing
		results = append(results, ChunkResult{
			Index:      i,
			StartTime:  chunkStart,
			EndTime:    chunkEnd,
			Text:       result.Text,
			Timestamps: result.Timestamps,
			Tokens:     result.Tokens,
		})

		fmt.Fprintf(os.Stderr, "OK (%d chars)\n", len(result.Text))
	}

	elapsed := time.Since(startTime)
	fmt.Fprintf(os.Stderr, "\nDone in %v (%.1fx realtime)\n", elapsed, duration/elapsed.Seconds())

	// Output in requested format
	switch outputFormat {
	case "json":
		outputJSON(results, duration, elapsed)
	case "vtt":
		outputVTT(results)
	default:
		outputText(results)
	}
}

func outputText(results []ChunkResult) {
	var b strings.Builder
	for i, r := range results {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(r.Text)
	}
	fmt.Println(b.String())
}

func outputJSON(results []ChunkResult, duration float64, elapsed time.Duration) {
	type Output struct {
		Duration  float64       `json:"duration_seconds"`
		Chunks    int           `json:"chunks"`
		Text      string        `json:"text"`
		ChunkData []ChunkResult `json:"chunk_data,omitempty"`
	}

	var fullText strings.Builder
	for i, r := range results {
		if i > 0 {
			fullText.WriteString(" ")
		}
		fullText.WriteString(r.Text)
	}

	out := Output{
		Duration:  duration,
		Chunks:    len(results),
		Text:      fullText.String(),
		ChunkData: results,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}

func outputVTT(results []ChunkResult) {
	fmt.Println("WEBVTT")
	fmt.Println()

	cueNum := 1
	for _, r := range results {
		// Group tokens into cues based on natural breaks or time gaps
		cues := groupTokensIntoCues(r)

		for _, cue := range cues {
			// Adjust timestamps by chunk start time
			start := r.StartTime + cue.Start
			end := r.StartTime + cue.End

			// Skip empty cues
			if strings.TrimSpace(cue.Text) == "" {
				continue
			}

			fmt.Printf("%d\n", cueNum)
			fmt.Printf("%s --> %s\n", formatVTTTime(start), formatVTTTime(end))
			fmt.Println(cue.Text)
			fmt.Println()

			cueNum++
		}
	}
}

type Cue struct {
	Start float64
	End   float64
	Text  string
}

func groupTokensIntoCues(r ChunkResult) []Cue {
	if len(r.Tokens) == 0 {
		return []Cue{{Start: 0, End: r.EndTime - r.StartTime, Text: r.Text}}
	}

	var cues []Cue
	var current Cue

	for i, tok := range r.Tokens {
		if i >= len(r.Timestamps) {
			break
		}

		timestamp := float64(r.Timestamps[i])

		// Start new cue if needed
		if current.Text == "" {
			current.Start = timestamp
		}

		current.Text += tok
		current.End = timestamp

		// End cue on sentence boundary or after ~5 seconds
		if isSentenceEnd(tok) || (current.End-current.Start > 5.0) {
			current.Text = strings.TrimSpace(current.Text)
			if current.Text != "" {
				cues = append(cues, current)
			}
			current = Cue{}
		}
	}

	// Add remaining
	if current.Text != "" {
		current.Text = strings.TrimSpace(current.Text)
		cues = append(cues, current)
	}

	return cues
}

func isSentenceEnd(tok string) bool {
	t := strings.TrimSpace(tok)
	return strings.HasSuffix(t, ".") || strings.HasSuffix(t, "!") || strings.HasSuffix(t, "?")
}

func formatVTTTime(seconds float64) string {
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	ms := int((seconds - float64(int(seconds))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
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

func transcribeChunk(recognizer *asr.Recognizer, audioFile string, start, duration float64) (*asr.Result, error) {
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "chough-*")
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("ffmpeg: %s", out)
	}

	// Transcribe with loaded model
	result, err := recognizer.Transcribe(chunkFile)
	if err != nil {
		return nil, err
	}

	return result, nil
}
