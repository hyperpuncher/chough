package transcribe

import (
	"fmt"
	"math"
)

// MinChunkDuration is the shortest useful independent audio chunk.
const MinChunkDuration = 0.5

// BuildBoundaries creates chunk boundaries while merging a tiny final
// remainder into the preceding chunk.
func BuildBoundaries(duration float64, chunkSize int) ([]float64, error) {
	if math.IsNaN(duration) || math.IsInf(duration, 0) || duration < 0 {
		return nil, fmt.Errorf("invalid audio duration: %v", duration)
	}
	if chunkSize <= 0 {
		return nil, fmt.Errorf("chunk size must be positive")
	}
	if duration == 0 {
		return []float64{0}, nil
	}

	boundaries := []float64{0}
	for boundary := float64(chunkSize); boundary < duration; boundary += float64(chunkSize) {
		boundaries = append(boundaries, boundary)
	}

	last := boundaries[len(boundaries)-1]
	if len(boundaries) > 1 && duration-last < MinChunkDuration {
		boundaries = boundaries[:len(boundaries)-1]
	}

	return append(boundaries, duration), nil
}
