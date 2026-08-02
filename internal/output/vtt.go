package output

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/hyperpuncher/chough/internal/types"
)

const (
	maximumCueDuration = 5.0
	minimumCueDuration = 0.001
)

// Cue represents a subtitle cue for VTT.
type Cue struct {
	Start float64
	End   float64
	Text  string
}

// WriteVTT writes WebVTT output.
func WriteVTT(out io.Writer, results []types.ChunkResult) error {
	if _, err := fmt.Fprintln(out, "WEBVTT"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}

	cueNum := 1
	for _, result := range results {
		for _, cue := range GroupTokensIntoCues(result) {
			text := strings.TrimSpace(cue.Text)
			if text == "" {
				continue
			}

			start := result.StartTime + cue.Start
			end := result.StartTime + cue.End
			startMillis := vttMilliseconds(start)
			endMillis := vttMilliseconds(end)
			if endMillis <= startMillis {
				endMillis = startMillis + 1
			}

			if _, err := fmt.Fprintf(out, "%d\n", cueNum); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(out, "%s --> %s\n", formatVTTMilliseconds(startMillis), formatVTTMilliseconds(endMillis)); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(out, text); err != nil {
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

// GroupTokensIntoCues groups tokens into subtitle cues.
func GroupTokensIntoCues(result types.ChunkResult) []Cue {
	chunkDuration := result.EndTime - result.StartTime
	if chunkDuration <= 0 || math.IsNaN(chunkDuration) {
		return nil
	}
	if len(result.Tokens) == 0 || len(result.Timestamps) == 0 {
		text := strings.TrimSpace(result.Text)
		if text == "" {
			return nil
		}
		return []Cue{{Start: 0, End: chunkDuration, Text: text}}
	}

	tokenCount := min(len(result.Tokens), len(result.Timestamps))
	cues := make([]Cue, 0)
	current := Cue{}
	active := false

	for i := 0; i < tokenCount; i++ {
		start, end := tokenInterval(result, i, tokenCount, chunkDuration)
		if active && end-current.Start > maximumCueDuration {
			if cue, ok := finishCue(current, chunkDuration); ok {
				cues = append(cues, cue)
			}
			current = Cue{}
			active = false
		}
		if !active {
			current.Start = start
			active = true
		}
		current.Text += result.Tokens[i]
		current.End = end

		if IsSentenceEnd(result.Tokens[i]) {
			if cue, ok := finishCue(current, chunkDuration); ok {
				cues = append(cues, cue)
			}
			current = Cue{}
			active = false
		}
	}
	if active {
		if cue, ok := finishCue(current, chunkDuration); ok {
			cues = append(cues, cue)
		}
	}
	return cues
}

func tokenInterval(result types.ChunkResult, index, tokenCount int, chunkDuration float64) (float64, float64) {
	start := clampTime(float64(result.Timestamps[index]), chunkDuration)
	end := chunkDuration
	if len(result.Durations) == len(result.Tokens) {
		duration := float64(result.Durations[index])
		if duration > 0 && !math.IsNaN(duration) && !math.IsInf(duration, 0) {
			end = start + duration
		} else if index+1 < tokenCount {
			end = float64(result.Timestamps[index+1])
		}
	} else if index+1 < tokenCount {
		end = float64(result.Timestamps[index+1])
	}

	if index+1 < tokenCount {
		nextStart := clampTime(float64(result.Timestamps[index+1]), chunkDuration)
		if nextStart > start && end > nextStart {
			end = nextStart
		}
	}
	end = clampTime(end, chunkDuration)
	if end > start {
		return start, end
	}
	if start < chunkDuration {
		return start, min(start+minimumCueDuration, chunkDuration)
	}
	return max(0, chunkDuration-minimumCueDuration), chunkDuration
}

func finishCue(cue Cue, chunkDuration float64) (Cue, bool) {
	cue.Text = strings.TrimSpace(cue.Text)
	if cue.Text == "" {
		return Cue{}, false
	}
	cue.Start = clampTime(cue.Start, chunkDuration)
	cue.End = clampTime(cue.End, chunkDuration)
	if cue.End <= cue.Start {
		if cue.Start < chunkDuration {
			cue.End = min(cue.Start+minimumCueDuration, chunkDuration)
		} else {
			cue.Start = max(0, chunkDuration-minimumCueDuration)
			cue.End = chunkDuration
		}
	}
	return cue, cue.End > cue.Start
}

func clampTime(value, maximum float64) float64 {
	if math.IsNaN(value) || value < 0 {
		return 0
	}
	if math.IsInf(value, 1) || value > maximum {
		return maximum
	}
	return value
}

// IsSentenceEnd checks if a token ends a sentence.
func IsSentenceEnd(token string) bool {
	trimmed := strings.TrimSpace(token)
	return strings.HasSuffix(trimmed, ".") || strings.HasSuffix(trimmed, "!") || strings.HasSuffix(trimmed, "?")
}

// FormatVTTTime formats seconds as a WebVTT timestamp.
func FormatVTTTime(seconds float64) string {
	return formatVTTMilliseconds(vttMilliseconds(seconds))
}

func vttMilliseconds(seconds float64) int64 {
	if seconds <= 0 || math.IsNaN(seconds) {
		return 0
	}
	if math.IsInf(seconds, 1) {
		return math.MaxInt64
	}
	return int64(math.Round(seconds * 1000))
}

func formatVTTMilliseconds(total int64) string {
	hours := total / 3_600_000
	minutes := total / 60_000 % 60
	seconds := total / 1000 % 60
	milliseconds := total % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, milliseconds)
}
