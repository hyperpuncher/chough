package output

import (
	"fmt"
	"io"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/hyperpuncher/chough/internal/tokens"
	"github.com/hyperpuncher/chough/internal/types"
)

const (
	// Netflix permits cues from 5/6 second through 7 seconds. BBC broadcast
	// subtitles use at most two 37-character lines and target 180 WPM.
	minimumCueDuration       = 5.0 / 6.0
	maximumCueDuration       = 7.0
	minimumTimestampDuration = 0.001
	maximumLineCharacters    = 37
	maximumCueCharacters     = 2 * maximumLineCharacters
	wordsPerSecond           = 3.0
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
	var previousEndMillis int64
	for _, result := range resultsForVTT(results) {
		for _, cue := range GroupTokensIntoCues(result) {
			text := strings.TrimSpace(cue.Text)
			if text == "" {
				continue
			}

			start := result.StartTime + cue.Start
			end := result.StartTime + cue.End
			startMillis := max(vttMilliseconds(start), previousEndMillis)
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

			previousEndMillis = endMillis
			cueNum++
		}
	}

	return nil
}

type cueUnit struct {
	text        string
	start       float64
	end         float64
	sentenceEnd bool
	clauseEnd   bool
}

type groupedCue struct {
	Cue
	wordCount int
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
	tokenValues := result.Tokens[:tokenCount]
	usesWordBoundaries := false
	for _, token := range tokenValues {
		if tokens.StartsWord(token) {
			usesWordBoundaries = true
			break
		}
	}

	units := make([]cueUnit, 0, tokenCount)
	for _, unit := range tokens.Units(tokenValues) {
		start, _ := tokenInterval(result, unit.Start, tokenCount, chunkDuration)
		_, end := tokenInterval(result, unit.End, tokenCount, chunkDuration)
		text := strings.Join(tokenValues[unit.Start:unit.End+1], "")
		units = append(units, cueUnit{
			text:        text,
			start:       start,
			end:         end,
			sentenceEnd: IsSentenceEnd(text),
			clauseEnd:   isClauseEnd(text),
		})
	}

	grouped := groupCueUnits(units, usesWordBoundaries)
	cues := make([]Cue, len(grouped))
	for index := range grouped {
		cues[index] = grouped[index].Cue
	}
	adjustCueTimings(cues, grouped, chunkDuration)
	return cues
}

func groupCueUnits(units []cueUnit, countWords bool) []groupedCue {
	grouped := make([]groupedCue, 0, len(units))
	for first := 0; first < len(units); {
		last := first
		lastClause := -1
		for next := first; next < len(units); next++ {
			text := cueUnitText(units, first, next)
			exceedsDuration := units[next].end-units[first].start > maximumCueDuration
			exceedsLength := utf8.RuneCountInString(text) > maximumCueCharacters
			if next > first && (exceedsDuration || exceedsLength) {
				if lastClause >= first {
					last = lastClause
				}
				break
			}

			last = next
			if units[next].clauseEnd {
				lastClause = next
			}
			if units[next].sentenceEnd {
				break
			}
		}

		wordCount := 0
		if countWords {
			wordCount = last - first + 1
		}
		grouped = append(grouped, groupedCue{
			Cue: Cue{
				Start: units[first].start,
				End:   min(units[last].end, units[first].start+maximumCueDuration),
				Text:  formatCueLines(units, first, last),
			},
			wordCount: wordCount,
		})
		first = last + 1
	}
	return grouped
}

func cueUnitText(units []cueUnit, first, last int) string {
	var text strings.Builder
	for index := first; index <= last; index++ {
		text.WriteString(units[index].text)
	}
	return strings.TrimSpace(text.String())
}

func formatCueLines(units []cueUnit, first, last int) string {
	text := cueUnitText(units, first, last)
	if first == last || utf8.RuneCountInString(text) <= maximumLineCharacters {
		return text
	}

	bestBreak := first
	bestPunctuation := false
	bestOverflow := math.MaxInt
	bestImbalance := math.MaxInt
	for index := first; index < last; index++ {
		left := cueUnitText(units, first, index)
		right := cueUnitText(units, index+1, last)
		leftLength := utf8.RuneCountInString(left)
		rightLength := utf8.RuneCountInString(right)
		overflow := max(0, leftLength-maximumLineCharacters) + max(0, rightLength-maximumLineCharacters)
		punctuation := units[index].sentenceEnd || units[index].clauseEnd
		imbalance := max(leftLength, rightLength) - min(leftLength, rightLength)
		if overflow < bestOverflow ||
			(overflow == bestOverflow && punctuation && !bestPunctuation) ||
			(overflow == bestOverflow && punctuation == bestPunctuation && imbalance < bestImbalance) {
			bestBreak = index
			bestPunctuation = punctuation
			bestOverflow = overflow
			bestImbalance = imbalance
		}
	}
	return cueUnitText(units, first, bestBreak) + "\n" + cueUnitText(units, bestBreak+1, last)
}

func adjustCueTimings(cues []Cue, grouped []groupedCue, chunkDuration float64) {
	for index := range cues {
		availableEnd := chunkDuration
		if index+1 < len(cues) {
			availableEnd = cues[index+1].Start
		}
		readableDuration := minimumCueDuration
		if grouped[index].wordCount > 0 {
			readableDuration = max(readableDuration, float64(grouped[index].wordCount)/wordsPerSecond)
		}
		desiredEnd := cues[index].Start + min(readableDuration, maximumCueDuration)
		cues[index].End = min(max(cues[index].End, desiredEnd), availableEnd, cues[index].Start+maximumCueDuration)
		if cues[index].End <= cues[index].Start {
			cues[index].End = min(cues[index].Start+minimumTimestampDuration, availableEnd)
		}
	}
}

func isClauseEnd(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasSuffix(trimmed, ",") || strings.HasSuffix(trimmed, ";") ||
		strings.HasSuffix(trimmed, ":") || strings.HasSuffix(trimmed, "—") ||
		strings.HasSuffix(trimmed, "–") || strings.HasSuffix(trimmed, "，") ||
		strings.HasSuffix(trimmed, "；") || strings.HasSuffix(trimmed, "：")
}

func resultsForVTT(results []types.ChunkResult) []types.ChunkResult {
	combined := types.ChunkResult{}
	hasTokens := false
	hasDurations := true
	for _, result := range results {
		if len(result.Tokens) == 0 && strings.TrimSpace(result.Text) == "" {
			continue
		}
		if len(result.Tokens) == 0 || len(result.Timestamps) != len(result.Tokens) {
			return results
		}
		if !hasTokens {
			combined.StartTime = result.StartTime
		}
		hasTokens = true
		combined.EndTime = max(combined.EndTime, result.EndTime)
		combined.Text += result.Text
		combined.Tokens = append(combined.Tokens, result.Tokens...)
		for _, timestamp := range result.Timestamps {
			combined.Timestamps = append(combined.Timestamps, float32(result.StartTime-combined.StartTime)+timestamp)
		}
		if len(result.Durations) == len(result.Tokens) {
			combined.Durations = append(combined.Durations, result.Durations...)
		} else {
			hasDurations = false
		}
	}
	if !hasTokens {
		return results
	}
	if !hasDurations {
		combined.Durations = nil
	}
	return []types.ChunkResult{combined}
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
		return start, min(start+minimumTimestampDuration, chunkDuration)
	}
	return max(0, chunkDuration-minimumTimestampDuration), chunkDuration
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
	return strings.HasSuffix(trimmed, ".") || strings.HasSuffix(trimmed, "!") ||
		strings.HasSuffix(trimmed, "?") || strings.HasSuffix(trimmed, "。") ||
		strings.HasSuffix(trimmed, "！") || strings.HasSuffix(trimmed, "？")
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
