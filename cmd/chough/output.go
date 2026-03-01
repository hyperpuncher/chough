package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ChunkResult holds transcription result for a single chunk.
type ChunkResult struct {
	StartTime  float64
	EndTime    float64
	Text       string
	Timestamps []float32
	Tokens     []string
}

// Cue represents a subtitle cue for VTT.
type Cue struct {
	Start float64
	End   float64
	Text  string
}

func writeOutput(out io.Writer, format string, results []ChunkResult, duration float64) error {
	switch format {
	case "json":
		return outputJSON(out, results, duration)
	case "vtt":
		return outputVTT(out, results)
	default:
		return outputText(out, results)
	}
}

func fullText(results []ChunkResult) string {
	parts := make([]string, 0, len(results))
	for _, r := range results {
		if strings.TrimSpace(r.Text) == "" {
			continue
		}
		parts = append(parts, r.Text)
	}
	return strings.Join(parts, " ")
}

func outputText(out io.Writer, results []ChunkResult) error {
	_, err := fmt.Fprintln(out, fullText(results))
	return err
}

func outputJSON(out io.Writer, results []ChunkResult, duration float64) error {
	type Output struct {
		Duration  float64       `json:"duration_seconds"`
		Chunks    int           `json:"chunks"`
		Text      string        `json:"text"`
		ChunkData []ChunkResult `json:"chunk_data,omitempty"`
	}

	data := Output{
		Duration:  duration,
		Chunks:    len(results),
		Text:      fullText(results),
		ChunkData: results,
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func outputVTT(out io.Writer, results []ChunkResult) error {
	if _, err := fmt.Fprintln(out, "WEBVTT"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}

	cueNum := 1
	for _, r := range results {
		for _, cue := range groupTokensIntoCues(r) {
			if strings.TrimSpace(cue.Text) == "" {
				continue
			}

			start := r.StartTime + cue.Start
			end := r.StartTime + cue.End

			if _, err := fmt.Fprintf(out, "%d\n", cueNum); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(out, "%s --> %s\n", formatVTTTime(start), formatVTTTime(end)); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(out, cue.Text); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}

			cueNum++
		}
	}

	return nil
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
		if current.Text == "" {
			current.Start = timestamp
		}

		current.Text += tok
		current.End = timestamp

		if isSentenceEnd(tok) || (current.End-current.Start > 5.0) {
			current.Text = strings.TrimSpace(current.Text)
			if current.Text != "" {
				cues = append(cues, current)
			}
			current = Cue{}
		}
	}

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
