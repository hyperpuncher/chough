package types

// ChunkResult holds transcription result for a single chunk
type ChunkResult struct {
	StartTime  float64   `json:"start_time"`
	EndTime    float64   `json:"end_time"`
	Text       string    `json:"text"`
	Timestamps []float32 `json:"timestamps,omitempty"`
	Tokens     []string  `json:"tokens,omitempty"`
	Durations  []float32 `json:"durations,omitempty"`
	LogProbs   []float32 `json:"log_probs,omitempty"`
}
