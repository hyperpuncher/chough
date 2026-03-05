package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/hyperpuncher/chough/internal/types"
)

// Cue represents a subtitle cue for VTT
type Cue struct {
	Start float64
	End   float64
	Text  string
}

// WriteVTT writes WebVTT output
func WriteVTT(out io.Writer, results []types.ChunkResult) error {
	if _, err := fmt.Fprintln(out, "WEBVTT"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}

	cueNum := 1
	for _, r := range results {
		for _, cue := range GroupTokensIntoCues(r) {
			if strings.TrimSpace(cue.Text) == "" {
				continue
			}

			start := r.StartTime + cue.Start
			end := r.StartTime + cue.End

			if _, err := fmt.Fprintf(out, "%d\n", cueNum); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(out, "%s --> %s\n", FormatVTTTime(start), FormatVTTTime(end)); err != nil {
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

// GroupTokensIntoCues groups tokens into subtitle cues
func GroupTokensIntoCues(r types.ChunkResult) []Cue {
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

		if IsSentenceEnd(tok) || (current.End-current.Start > 5.0) {
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

// IsSentenceEnd checks if a token ends a sentence
func IsSentenceEnd(tok string) bool {
	t := strings.TrimSpace(tok)
	return strings.HasSuffix(t, ".") || strings.HasSuffix(t, "!") || strings.HasSuffix(t, "?")
}

// FormatVTTTime formats seconds as WebVTT timestamp
func FormatVTTTime(seconds float64) string {
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	ms := int((seconds - float64(int(seconds))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}
