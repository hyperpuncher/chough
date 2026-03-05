package output

import (
	"io"

	"github.com/hyperpuncher/chough/internal/types"
)

// Write writes formatted output to the given writer
func Write(out io.Writer, format string, results []types.ChunkResult, duration float64) error {
	switch format {
	case "json":
		return WriteJSON(out, results, duration)
	case "vtt":
		return WriteVTT(out, results)
	default:
		return WriteText(out, results)
	}
}
