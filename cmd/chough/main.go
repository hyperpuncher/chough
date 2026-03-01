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
	"unicode/utf8"

	"github.com/hyperpuncher/chough/internal/asr"
	"github.com/hyperpuncher/chough/internal/models"
)

// ANSI color codes
const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	gray    = "\033[38;5;250m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
)

// renderProgressBar creates a visual progress bar
func renderProgressBar(current, total, width int) string {
	if total == 0 {
		return ""
	}
	filled := (current * width) / total
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("%s%s%s", gray, bar, reset)
}

// formatETA formats duration as human readable time (e.g., "1m 23s" or "45s")
func formatETA(d time.Duration) string {
	if d < 0 {
		return "0s"
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// ChunkResult holds transcription result for a single chunk
type ChunkResult struct {
	StartTime  float64
	EndTime    float64
	Text       string
	Timestamps []float32
	Tokens     []string
}

// Cue represents a subtitle cue for VTT
type Cue struct {
	Start float64
	End   float64
	Text  string
}

var version = "dev"

type cliFlag struct {
	short       string
	long        string
	arg         string
	description string
	defaultVal  string
}

func formatFlagLabel(f cliFlag) string {
	parts := make([]string, 0, 2)
	if f.short != "" {
		parts = append(parts, "-"+f.short)
	}
	if f.long != "" {
		parts = append(parts, "--"+f.long)
	}

	return strings.Join(parts, ", ")
}

func plainFlagLabel(f cliFlag) string {
	label := formatFlagLabel(f)
	if f.arg != "" {
		label += " " + f.arg
	}
	return label
}

func coloredFlagLabel(f cliFlag) string {
	base := formatFlagLabel(f)
	if f.arg == "" {
		return cyan + base + reset
	}
	return cyan + base + " " + yellow + f.arg + reset
}

type usageRow struct {
	label      string
	plainLabel string
	desc       string
}

func printAlignedRows(rows []usageRow) {
	maxLabelWidth := 0
	for _, row := range rows {
		if w := utf8.RuneCountInString(row.plainLabel); w > maxLabelWidth {
			maxLabelWidth = w
		}
	}

	for _, row := range rows {
		pad := strings.Repeat(" ", maxLabelWidth-utf8.RuneCountInString(row.plainLabel))
		fmt.Fprintf(os.Stderr, "  %s%s  %s\n", row.label, pad, row.desc)
	}
}

func printUsage(flags []cliFlag) {
	fmt.Fprintf(os.Stderr, "%s🐦‍⬛ %schough%s\n\n", bold, magenta, reset)
	fmt.Fprintf(os.Stderr, "%sUsage:%s\n", bold, reset)
	fmt.Fprintln(os.Stderr, "  chough [flags] <audio-file>")
	fmt.Fprintln(os.Stderr)

	fmt.Fprintf(os.Stderr, "%sFlags:%s\n", bold, reset)
	flagRows := make([]usageRow, 0, len(flags))
	for _, f := range flags {
		desc := f.description
		if f.defaultVal != "" {
			desc += fmt.Sprintf(" %s(default: %s)%s", dim, f.defaultVal, reset)
		}

		flagRows = append(flagRows, usageRow{
			label:      coloredFlagLabel(f),
			plainLabel: plainFlagLabel(f),
			desc:       desc,
		})
	}
	printAlignedRows(flagRows)
	fmt.Fprintln(os.Stderr)

	fmt.Fprintf(os.Stderr, "%sExamples:%s\n", bold, reset)
	exampleRows := []usageRow{
		{label: fmt.Sprintf("%s$%s chough audio.mp3", green, reset), plainLabel: "$ chough audio.mp3", desc: fmt.Sprintf("%s# 60s chunks, text output%s", dim, reset)},
		{label: fmt.Sprintf("%s$%s chough -c 30 talk.mp3", green, reset), plainLabel: "$ chough -c 30 talk.mp3", desc: fmt.Sprintf("%s# 30s chunks%s", dim, reset)},
		{label: fmt.Sprintf("%s$%s chough -f vtt -o subs.vtt audio.mp3", green, reset), plainLabel: "$ chough -f vtt -o subs.vtt audio.mp3", desc: fmt.Sprintf("%s# WebVTT to file%s", dim, reset)},
	}
	printAlignedRows(exampleRows)
	fmt.Fprintln(os.Stderr)

	fmt.Fprintf(os.Stderr, "%sEnvironment:%s\n", bold, reset)
	envRows := []usageRow{
		{label: fmt.Sprintf("%sCHOUGH_MODEL%s", cyan, reset), plainLabel: "CHOUGH_MODEL", desc: fmt.Sprintf("path to model dir %s(optional, auto-downloaded if not set)%s", dim, reset)},
	}
	printAlignedRows(envRows)
}

func main() {
	// Define flags
	usageFlags := []cliFlag{
		{short: "c", long: "chunk-size", arg: "int", description: "chunk size in seconds", defaultVal: "60"},
		{short: "f", long: "format", arg: "string", description: "output format: text, json, vtt", defaultVal: "text"},
		{short: "o", long: "output", arg: "file", description: "output file", defaultVal: "stdout"},
		{long: "version", description: "show version"},
	}

	chunkSize := flag.Int("c", 60, "chunk size in seconds")
	flag.IntVar(chunkSize, "chunk-size", 60, "chunk size in seconds")
	format := flag.String("f", "text", "output format (text, json, vtt)")
	flag.StringVar(format, "format", "text", "output format (text, json, vtt)")
	outputFile := flag.String("o", "", "output file")
	flag.StringVar(outputFile, "output", "", "output file")
	showVersion := flag.Bool("version", false, "show version")

	flag.Usage = func() {
		printUsage(usageFlags)
	}

	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

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

	// Load model (auto-download if needed)
	fmt.Fprint(os.Stderr, "\033[?25l⏳ Loading model...\r")
	modelPath, err := models.GetModelPath()
	if err != nil {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "Failed to get model: %v\n", err)
		os.Exit(1)
	}
	config := asr.Config{
		ModelPath:  modelPath,
		NumThreads: 4,
		SampleRate: 16000,
		FeatureDim: 80,
		Provider:   "cpu",
	}

	recognizer, err := asr.NewRecognizer(&config)
	if err != nil {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "Failed to load model: %v\n", err)
		os.Exit(1)
	}
	defer recognizer.Close()
	fmt.Fprintf(os.Stderr, "\r\033[?25h✅ Model loaded!   \n")

	// Get audio duration
	duration, err := getDuration(audioFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get duration: %v\n", err)
		os.Exit(1)
	}

	// Build fixed chunk boundaries
	chunkCount := int(duration/float64(chunkSecs)) + 1
	var boundaries []float64
	for i := 0; i < chunkCount; i++ {
		start := float64(i * chunkSecs)
		if start >= duration {
			break
		}
		boundaries = append(boundaries, start)
	}
	boundaries = append(boundaries, duration)

	fmt.Fprintf(os.Stderr, "audio: %.1fs %s•%s chunks: %ds %s•%s format: %s\n",
		duration, dim, reset, chunkSecs, dim, reset, outputFormat)

	// Process chunks
	startTime := time.Now()
	var results []ChunkResult

	// Hide cursor during progress
	fmt.Fprint(os.Stderr, "\033[?25l")

	for i := 0; i < len(boundaries)-1; i++ {
		chunkStart := boundaries[i]
		chunkEnd := boundaries[i+1]

		if chunkEnd-chunkStart < 0.5 { // Skip chunks smaller than 0.5s
			continue
		}

		// Progress bar with ETA
		progress := renderProgressBar(i+1, len(boundaries)-1, 40)
		elapsed := time.Since(startTime)
		percent := float64(i+1) / float64(len(boundaries)-1)
		eta := time.Duration(float64(elapsed)/percent - float64(elapsed))
		fmt.Fprintf(os.Stderr, "\r%s %sETA %s%s\033[K",
			progress, dim, formatETA(eta), reset)

		result, err := transcribeChunk(recognizer, audioFile, chunkStart, chunkEnd-chunkStart)
		if err != nil {
			// Show cursor on error
			fmt.Fprint(os.Stderr, "\033[?25h")
			elapsed := time.Since(startTime)
			fmt.Fprintf(os.Stderr, "\r%s %sETA %s ERR: %v%s\n",
				renderProgressBar(i+1, len(boundaries)-1, 40), dim, formatETA(elapsed), err, reset)
			continue
		}

		// Store result with timing
		results = append(results, ChunkResult{
			StartTime:  chunkStart,
			EndTime:    chunkEnd,
			Text:       result.Text,
			Timestamps: result.Timestamps,
			Tokens:     result.Tokens,
		})
	}

	// Final progress bar at 100% - show cursor
	fmt.Fprintf(os.Stderr, "\r\033[?25h%s %sETA 0s\033[K\n",
		renderProgressBar(len(boundaries)-1, len(boundaries)-1, 40), dim)

	elapsed := time.Since(startTime)
	rtFactor := duration / elapsed.Seconds()
	rtColor := green
	if rtFactor < 10 {
		rtColor = yellow
	}

	fmt.Fprintf(os.Stderr, "%s⚡%s Processed in %s%.1fs%s %s(%s%.1fx%s realtime)%s\n\n",
		yellow, reset, bold, elapsed.Seconds(), reset, dim, rtColor, rtFactor, reset, reset)

	// Determine output destination
	var out *os.File
	if *outputFile != "" {
		out, err = os.Create(*outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
			os.Exit(1)
		}
		defer out.Close()
		fmt.Fprintf(os.Stderr, "Output: %s\n", *outputFile)
	} else {
		out = os.Stdout
	}

	// Output in requested format
	switch outputFormat {
	case "json":
		outputJSON(out, results, duration, elapsed)
	case "vtt":
		outputVTT(out, results)
	default:
		outputText(out, results)
	}
}

func outputText(out *os.File, results []ChunkResult) {
	var b strings.Builder
	for i, r := range results {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(r.Text)
	}
	fmt.Fprintln(out, b.String())
}

func outputJSON(out *os.File, results []ChunkResult, duration float64, elapsed time.Duration) {
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

	data := Output{
		Duration:  duration,
		Chunks:    len(results),
		Text:      fullText.String(),
		ChunkData: results,
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}

func outputVTT(out *os.File, results []ChunkResult) {
	fmt.Fprintln(out, "WEBVTT")
	fmt.Fprintln(out)

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

			fmt.Fprintf(out, "%d\n", cueNum)
			fmt.Fprintf(out, "%s --> %s\n", formatVTTTime(start), formatVTTTime(end))
			fmt.Fprintln(out, cue.Text)
			fmt.Fprintln(out)

			cueNum++
		}
	}
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
		"-vn", // disable video
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
