package worker

import (
	"context"

	"github.com/hyperpuncher/chough/internal/transcribe"
)

// Job contains the work and lifecycle hooks managed by Pool.
type Job struct {
	Context   context.Context
	FilePath  string
	ChunkSize int
	Cleanup   func()
	// Response must be buffered so workers can finish after caller cancellation.
	Response chan Response
}

// Response reports the final transcription result or error.
type Response struct {
	Transcription transcribe.Result
	Error         error
}
