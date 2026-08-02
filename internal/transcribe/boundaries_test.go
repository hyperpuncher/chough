package transcribe

import (
	"math"
	"reflect"
	"testing"
)

func TestBuildBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		duration  float64
		chunkSize int
		want      []float64
	}{
		{name: "exact multiple", duration: 120, chunkSize: 60, want: []float64{0, 60, 120}},
		{name: "merge short remainder", duration: 120.2, chunkSize: 60, want: []float64{0, 60, 120.2}},
		{name: "keep useful remainder", duration: 120.5, chunkSize: 60, want: []float64{0, 60, 120, 120.5}},
		{name: "short file", duration: 0.2, chunkSize: 60, want: []float64{0, 0.2}},
		{name: "zero duration", duration: 0, chunkSize: 60, want: []float64{0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildBoundaries(tt.duration, tt.chunkSize)
			if err != nil {
				t.Fatalf("BuildBoundaries() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("BuildBoundaries() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildBoundariesRejectsInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name      string
		duration  float64
		chunkSize int
	}{
		{name: "negative duration", duration: -1, chunkSize: 60},
		{name: "NaN duration", duration: math.NaN(), chunkSize: 60},
		{name: "infinite duration", duration: math.Inf(1), chunkSize: 60},
		{name: "zero chunk size", duration: 60, chunkSize: 0},
		{name: "negative chunk size", duration: 60, chunkSize: -1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := BuildBoundaries(tt.duration, tt.chunkSize); err == nil {
				t.Fatal("BuildBoundaries() error = nil, want validation error")
			}
		})
	}
}
