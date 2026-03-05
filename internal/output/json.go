package output

import (
	"encoding/json"
	"io"

	"github.com/hyperpuncher/chough/internal/types"
)

// WriteJSON writes JSON output
func WriteJSON(out io.Writer, results []types.ChunkResult, duration float64) error {
	type Output struct {
		Duration  float64             `json:"duration_seconds"`
		Chunks    int                 `json:"chunks"`
		Text      string              `json:"text"`
		ChunkData []types.ChunkResult `json:"chunk_data,omitempty"`
	}

	data := Output{
		Duration:  duration,
		Chunks:    len(results),
		Text:      FullText(results),
		ChunkData: results,
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
