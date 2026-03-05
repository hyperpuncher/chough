package audio

// BuildBoundaries creates time boundaries for chunking audio.
// If chunkSecs is <= 0, returns a single boundary [0, duration] (no chunking).
func BuildBoundaries(duration float64, chunkSecs int) []float64 {
	if chunkSecs <= 0 {
		return []float64{0, duration}
	}

	chunkCount := int(duration/float64(chunkSecs)) + 1
	boundaries := make([]float64, 0, chunkCount+1)

	for i := 0; i < chunkCount; i++ {
		start := float64(i * chunkSecs)
		if start >= duration {
			break
		}
		boundaries = append(boundaries, start)
	}

	return append(boundaries, duration)
}
