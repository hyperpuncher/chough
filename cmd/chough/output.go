package main

import (
	"io"

	"github.com/hyperpuncher/chough/internal/output"
	"github.com/hyperpuncher/chough/internal/types"
)

// ChunkResult is an alias for the shared type
type ChunkResult = types.ChunkResult

func writeOutput(out io.Writer, format string, results []ChunkResult, duration float64) error {
	return output.Write(out, format, results, duration)
}
