package transcribe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hyperpuncher/chough/internal/asr"
	"github.com/hyperpuncher/chough/internal/audio"
	"github.com/hyperpuncher/chough/internal/output"
	"github.com/hyperpuncher/chough/internal/types"
)

// Recognizer transcribes normalized WAV audio. Native recognition cannot be
// interrupted once a call has started.
type Recognizer interface {
	Transcribe(audioPath string) (*asr.Result, error)
}

// AudioProcessor probes and normalizes source audio.
type AudioProcessor interface {
	ProbeDuration(ctx context.Context, audioFile string) (float64, error)
	ExtractChunkWAV(ctx context.Context, audioFile, chunkFile string, start, duration float64) error
}

type ffmpegProcessor struct{}

func (ffmpegProcessor) ProbeDuration(ctx context.Context, audioFile string) (float64, error) {
	return audio.ProbeDuration(ctx, audioFile)
}

func (ffmpegProcessor) ExtractChunkWAV(ctx context.Context, audioFile, chunkFile string, start, duration float64) error {
	return audio.ExtractChunkWAV(ctx, audioFile, chunkFile, start, duration)
}

// Progress describes completed work in a transcription.
type Progress struct {
	Completed int
	Total     int
	Duration  float64
	Elapsed   time.Duration
}

// Options controls a transcription operation.
type Options struct {
	ChunkSize int
	Progress  func(Progress)
}

// Result contains the complete transcription and timing information.
type Result struct {
	Duration       float64
	ProcessingTime time.Duration
	Text           string
	Chunks         []types.ChunkResult
}

// Service runs the shared local and server transcription pipeline.
type Service struct {
	recognizer     Recognizer
	audio          AudioProcessor
	recognizerSlot chan struct{}
}

// NewService creates a transcription service. It uses ffmpeg unless a custom
// audio processor is supplied for testing.
func NewService(recognizer Recognizer, processors ...AudioProcessor) *Service {
	processor := AudioProcessor(ffmpegProcessor{})
	if len(processors) > 0 && processors[0] != nil {
		processor = processors[0]
	}
	return &Service{
		recognizer:     recognizer,
		audio:          processor,
		recognizerSlot: make(chan struct{}, 1),
	}
}

// Transcribe probes, chunks, normalizes, and transcribes an audio file. It
// fails the entire operation on the first chunk error and never returns a
// partial transcript.
func (s *Service) Transcribe(ctx context.Context, audioFile string, options Options) (Result, error) {
	started := time.Now()
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if s == nil || s.recognizer == nil {
		return Result{}, fmt.Errorf("recognizer is required")
	}
	if options.ChunkSize <= 0 {
		return Result{}, fmt.Errorf("chunk size must be positive")
	}

	duration, err := s.audio.ProbeDuration(ctx, audioFile)
	if err != nil {
		return Result{}, fmt.Errorf("probe audio duration: %w", err)
	}
	boundaries, err := BuildBoundaries(duration, options.ChunkSize)
	if err != nil {
		return Result{}, err
	}

	result := Result{Duration: duration}
	chunkCount := len(boundaries) - 1
	if options.Progress != nil {
		options.Progress(Progress{
			Total:    chunkCount,
			Duration: duration,
			Elapsed:  time.Since(started),
		})
	}
	if chunkCount == 0 {
		result.ProcessingTime = time.Since(started)
		return result, nil
	}

	tmpDir, err := os.MkdirTemp("", "chough-chunks-*")
	if err != nil {
		return Result{}, fmt.Errorf("create chunk directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	chunkFile := filepath.Join(tmpDir, "chunk.wav")
	result.Chunks = make([]types.ChunkResult, 0, chunkCount)

	for i := 0; i < chunkCount; i++ {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}

		chunkStart := boundaries[i]
		chunkEnd := boundaries[i+1]
		if err := s.audio.ExtractChunkWAV(ctx, audioFile, chunkFile, chunkStart, chunkEnd-chunkStart); err != nil {
			return Result{}, fmt.Errorf("prepare chunk %d [%.3fs-%.3fs]: %w", i+1, chunkStart, chunkEnd, err)
		}
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}

		chunk, transcribeErr := s.transcribeChunk(ctx, chunkFile)
		if transcribeErr != nil {
			return Result{}, fmt.Errorf("transcribe chunk %d [%.3fs-%.3fs]: %w", i+1, chunkStart, chunkEnd, transcribeErr)
		}
		if chunk == nil {
			return Result{}, fmt.Errorf("transcribe chunk %d [%.3fs-%.3fs]: recognizer returned no result", i+1, chunkStart, chunkEnd)
		}
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}

		result.Chunks = append(result.Chunks, types.ChunkResult{
			StartTime:  chunkStart,
			EndTime:    chunkEnd,
			Text:       chunk.Text,
			Timestamps: chunk.Timestamps,
			Tokens:     chunk.Tokens,
			Durations:  alignedTokenValues(chunk.Durations, chunk.Tokens),
			LogProbs:   alignedTokenValues(chunk.LogProbs, chunk.Tokens),
		})
		if options.Progress != nil {
			options.Progress(Progress{
				Completed: i + 1,
				Total:     chunkCount,
				Duration:  duration,
				Elapsed:   time.Since(started),
			})
		}
	}

	result.ProcessingTime = time.Since(started)
	result.Text = output.FullText(result.Chunks)
	return result, nil
}

func alignedTokenValues(values []float32, tokens []string) []float32 {
	if len(values) == 0 || len(values) != len(tokens) {
		return nil
	}
	return values
}

func (s *Service) transcribeChunk(ctx context.Context, chunkFile string) (*asr.Result, error) {
	select {
	case s.recognizerSlot <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-s.recognizerSlot }()
	return s.recognizer.Transcribe(chunkFile)
}
