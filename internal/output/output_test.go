package output

import (
	"bytes"
	"encoding/json"
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

func TestWriteJSON(t *testing.T) {
	var out bytes.Buffer
	if err := WriteJSON(&out, outputResults, 5); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	var got struct {
		Duration float64 `json:"duration_seconds"`
		Chunks   int     `json:"chunks"`
		Text     string  `json:"text"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Duration != 5 || got.Chunks != 3 || got.Text != "hello world" {
		t.Fatalf("unexpected JSON output: %+v", got)
	}
}

func TestWriteVTT(t *testing.T) {
	var out bytes.Buffer
	if err := WriteVTT(&out, outputResults); err != nil {
		t.Fatalf("WriteVTT() error = %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"WEBVTT\n",
		"00:00:00.000 --> 00:00:01.000\nHello world.",
		"00:00:02.000 --> 00:00:04.000\nworld",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("WriteVTT() output missing %q:\n%s", want, got)
		}
	}
}

func TestFormatVTTTime(t *testing.T) {
	if got, want := FormatVTTTime(3661.25), "01:01:01.250"; got != want {
		t.Fatalf("FormatVTTTime() = %q, want %q", got, want)
	}
}
