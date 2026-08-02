package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/hyperpuncher/chough/internal/types"
)

var outputResults = []types.ChunkResult{
	{StartTime: 0, EndTime: 2, Text: " hello ", Tokens: []string{"Hello", " world."}, Timestamps: []float32{0, 1}},
	{StartTime: 2, EndTime: 4, Text: "world"},
	{StartTime: 4, EndTime: 5, Text: "  "},
}

func TestFullText(t *testing.T) {
	if got, want := FullText(outputResults), "hello world"; got != want {
		t.Fatalf("FullText() = %q, want %q", got, want)
	}
}

func TestWriteTextHasOneTrailingNewline(t *testing.T) {
	var out bytes.Buffer
	if err := WriteText(&out, outputResults); err != nil {
		t.Fatalf("WriteText() error = %v", err)
	}
	if got, want := out.String(), "hello world\n"; got != want {
		t.Fatalf("WriteText() = %q, want %q", got, want)
	}
}

func TestWriteJSONPreservesSchemaAndAddsOptionalTokenData(t *testing.T) {
	results := []types.ChunkResult{{
		StartTime:  0,
		EndTime:    2,
		Text:       "hello",
		Tokens:     []string{"hello"},
		Timestamps: []float32{0},
		Durations:  []float32{0.4},
		LogProbs:   []float32{-0.2},
	}}
	var out bytes.Buffer
	if err := WriteJSON(&out, results, 2); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	var got struct {
		Duration  float64 `json:"duration_seconds"`
		Chunks    int     `json:"chunks"`
		Text      string  `json:"text"`
		ChunkData []struct {
			Durations []float32 `json:"durations"`
			LogProbs  []float32 `json:"log_probs"`
		} `json:"chunk_data"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Duration != 2 || got.Chunks != 1 || got.Text != "hello" {
		t.Fatalf("unexpected existing JSON fields: %+v", got)
	}
	if len(got.ChunkData) != 1 || len(got.ChunkData[0].Durations) != 1 || len(got.ChunkData[0].LogProbs) != 1 {
		t.Fatalf("optional token data missing: %+v", got.ChunkData)
	}
}

func TestWriteJSONOmitsUnavailableTokenData(t *testing.T) {
	var out bytes.Buffer
	if err := WriteJSON(&out, []types.ChunkResult{{Text: "hello"}}, 1); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	if strings.Contains(out.String(), "durations") || strings.Contains(out.String(), "log_probs") {
		t.Fatalf("unavailable fields were not omitted:\n%s", out.String())
	}
}

func TestWriteVTTUsesFinalTokenFallback(t *testing.T) {
	var out bytes.Buffer
	if err := WriteVTT(&out, outputResults); err != nil {
		t.Fatalf("WriteVTT() error = %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"WEBVTT\n",
		"00:00:00.000 --> 00:00:02.000\nHello world.",
		"00:00:02.000 --> 00:00:04.000\nworld",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("WriteVTT() output missing %q:\n%s", want, got)
		}
	}
}

func TestGroupTokensIntoCuesUsesTokenDuration(t *testing.T) {
	cues := GroupTokensIntoCues(types.ChunkResult{
		StartTime:  10,
		EndTime:    12,
		Tokens:     []string{"hello"},
		Timestamps: []float32{1},
		Durations:  []float32{0.4},
	})
	if len(cues) != 1 || cues[0].Start != 1 || !near(cues[0].End, 1.4) {
		t.Fatalf("cues = %+v, want 1.0-1.4", cues)
	}
}

func TestGroupTokensIntoCuesGuaranteesPositiveSingleTokenCue(t *testing.T) {
	cues := GroupTokensIntoCues(types.ChunkResult{
		StartTime:  0,
		EndTime:    2,
		Tokens:     []string{"hello"},
		Timestamps: []float32{2},
	})
	if len(cues) != 1 || cues[0].Start < 0 || cues[0].End > 2 || cues[0].End <= cues[0].Start {
		t.Fatalf("invalid single-token cue: %+v", cues)
	}
}

func TestGroupTokensIntoCuesSplitsSentencesWithoutOverlap(t *testing.T) {
	cues := GroupTokensIntoCues(types.ChunkResult{
		StartTime:  0,
		EndTime:    4,
		Tokens:     []string{"First.", " Second."},
		Timestamps: []float32{0, 2},
		Durations:  []float32{3, 1},
	})
	if len(cues) != 2 || cues[0].End > cues[1].Start {
		t.Fatalf("sentence cues overlap: %+v", cues)
	}
	if cues[0].Text != "First." || cues[1].Text != "Second." {
		t.Fatalf("sentence cue text = %+v", cues)
	}
}

func TestGroupTokensIntoCuesSplitsLongCue(t *testing.T) {
	result := types.ChunkResult{StartTime: 0, EndTime: 8}
	for i := range 8 {
		result.Tokens = append(result.Tokens, " word")
		result.Timestamps = append(result.Timestamps, float32(i))
	}
	cues := GroupTokensIntoCues(result)
	if len(cues) != 2 || cues[0].End-cues[0].Start > maximumCueDuration || cues[1].End-cues[1].Start > maximumCueDuration {
		t.Fatalf("long cue split = %+v", cues)
	}
}

func TestFormatVTTTimeRoundsAcrossBoundaries(t *testing.T) {
	for _, test := range []struct {
		seconds float64
		want    string
	}{
		{seconds: 1.2344, want: "00:00:01.234"},
		{seconds: 1.2346, want: "00:00:01.235"},
		{seconds: 59.9996, want: "00:01:00.000"},
		{seconds: 3599.9996, want: "01:00:00.000"},
		{seconds: 3661.25, want: "01:01:01.250"},
	} {
		if got := FormatVTTTime(test.seconds); got != test.want {
			t.Fatalf("FormatVTTTime(%v) = %q, want %q", test.seconds, got, test.want)
		}
	}
}

func TestWriteRejectsUnknownFormat(t *testing.T) {
	err := Write(&bytes.Buffer{}, "yaml", nil, 0)
	if err == nil || !strings.Contains(err.Error(), "unsupported output format") {
		t.Fatalf("Write() error = %v", err)
	}
}

func TestWritersReportWriteErrors(t *testing.T) {
	for _, test := range []struct {
		name  string
		write func() error
	}{
		{name: "text", write: func() error { return WriteText(failingWriter{}, outputResults) }},
		{name: "json", write: func() error { return WriteJSON(failingWriter{}, outputResults, 5) }},
		{name: "vtt", write: func() error { return WriteVTT(failingWriter{}, outputResults) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.write(); !errors.Is(err, errWrite) {
				t.Fatalf("writer error = %v, want %v", err, errWrite)
			}
		})
	}
}

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errWrite
}

func near(left, right float64) bool {
	return left > right-0.0001 && left < right+0.0001
}
