package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/hyperpuncher/chough/internal/types"
)

// FullText extracts the full text from chunk results
func FullText(results []types.ChunkResult) string {
	parts := make([]string, 0, len(results))
	for _, r := range results {
		text := strings.TrimSpace(r.Text)
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, " ")
}

// WriteText writes plain text output
func WriteText(out io.Writer, results []types.ChunkResult) error {
	_, err := fmt.Fprintln(out, FullText(results))
	return err
}
