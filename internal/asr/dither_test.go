package asr

import (
	"reflect"
	"testing"
)

func TestApplyDitherIsDeterministic(t *testing.T) {
	first := make([]float32, 1024)
	second := make([]float32, len(first))
	applyDither(first, 0.001)
	applyDither(second, 0.001)

	if !reflect.DeepEqual(first, second) {
		t.Fatal("applyDither() produced different samples")
	}
	if reflect.DeepEqual(first, make([]float32, len(first))) {
		t.Fatal("applyDither() did not alter samples")
	}
}

func TestApplyDitherDisabled(t *testing.T) {
	samples := []float32{-1, -0.5, 0, 0.5, 1}
	want := append([]float32(nil), samples...)
	applyDither(samples, 0)
	if !reflect.DeepEqual(samples, want) {
		t.Fatalf("applyDither() = %v, want %v", samples, want)
	}
}

func TestApplyDitherClampsSamples(t *testing.T) {
	samples := []float32{-1, 1}
	for range 100 {
		applyDither(samples, 0.001)
		for _, sample := range samples {
			if sample < -1 || sample > 1 {
				t.Fatalf("applyDither() sample = %v", sample)
			}
		}
	}
}
