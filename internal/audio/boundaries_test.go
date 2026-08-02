package audio

import (
	"reflect"
	"testing"
)

func TestBuildBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		duration  float64
		chunkSecs int
		want      []float64
	}{
		{name: "even chunks", duration: 120, chunkSecs: 60, want: []float64{0, 60, 120}},
		{name: "partial final chunk", duration: 125, chunkSecs: 60, want: []float64{0, 60, 120, 125}},
		{name: "shorter than chunk", duration: 15, chunkSecs: 60, want: []float64{0, 15}},
		{name: "chunking disabled", duration: 125, chunkSecs: 0, want: []float64{0, 125}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildBoundaries(tt.duration, tt.chunkSecs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("BuildBoundaries(%v, %d) = %v, want %v", tt.duration, tt.chunkSecs, got, tt.want)
			}
		})
	}
}
