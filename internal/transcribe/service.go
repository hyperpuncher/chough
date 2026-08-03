package transcribe

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hyperpuncher/chough/internal/asr"
	"github.com/hyperpuncher/chough/internal/audio"
	"github.com/hyperpuncher/chough/internal/output"
	"github.com/hyperpuncher/chough/internal/tokens"
	"github.com/hyperpuncher/chough/internal/types"
)

// DefaultChunkSize is the CLI and HTTP API chunk duration in seconds.
const (
	DefaultChunkSize       = 20
	MaximumChunkSize       = 300
	maximumContextDuration = 2.0
)

// ValidateChunkSize rejects inference windows outside the tested safe range.
func ValidateChunkSize(size int) error {
	if size <= 0 {
		return fmt.Errorf("chunk size must be positive")
	}
	if size > MaximumChunkSize {
		return fmt.Errorf("chunk size must not exceed %d seconds", MaximumChunkSize)
	}
	return nil
}

// Recognizer transcribes normalized WAV audio. Native recognition cannot be
// interrupted once a call has started.
type Recognizer interface {
	Transcribe(audioPath string) (*asr.Result, error)
}

type timestampedRecognizer interface {
	SupportsTokenTimestamps() bool
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
	if err := ValidateChunkSize(options.ChunkSize); err != nil {
		return Result{}, err
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
	contextDuration := 0.0
	if recognizer, ok := s.recognizer.(timestampedRecognizer); ok && recognizer.SupportsTokenTimestamps() && chunkCount > 1 {
		// Keep every contextual inference window within MaximumChunkSize.
		availableContext := (float64(MaximumChunkSize) - float64(options.ChunkSize)) / 2
		contextDuration = max(0, min(maximumContextDuration, float64(options.ChunkSize)/4, availableContext))
	}
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
		audioStart := max(0, chunkStart-contextDuration)
		audioEnd := min(duration, chunkEnd+contextDuration)
		if err := s.audio.ExtractChunkWAV(ctx, audioFile, chunkFile, audioStart, audioEnd-audioStart); err != nil {
			return Result{}, fmt.Errorf("prepare chunk %d [%.3fs-%.3fs]: %w", i+1, audioStart, audioEnd, err)
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

		mapped, err := mapChunkResult(chunk, audioStart, chunkStart, chunkEnd, contextDuration > 0, i == chunkCount-1)
		if err != nil {
			return Result{}, fmt.Errorf("map chunk %d [%.3fs-%.3fs]: %w", i+1, chunkStart, chunkEnd, err)
		}
		result.Chunks = append(result.Chunks, mapped)
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

func mapChunkResult(result *asr.Result, audioStart, chunkStart, chunkEnd float64, trimContext, finalChunk bool) (types.ChunkResult, error) {
	mapped := types.ChunkResult{
		StartTime:  chunkStart,
		EndTime:    chunkEnd,
		Text:       result.Text,
		Timestamps: result.Timestamps,
		Tokens:     result.Tokens,
		Durations:  alignedTokenValues(result.Durations, result.Tokens),
		LogProbs:   alignedTokenValues(result.LogProbs, result.Tokens),
	}
	if !trimContext {
		return mapped, nil
	}
	if len(result.Tokens) == 0 && strings.TrimSpace(result.Text) == "" {
		return mapped, nil
	}
	if len(result.Tokens) == 0 || len(result.Timestamps) != len(result.Tokens) {
		return types.ChunkResult{}, fmt.Errorf("recognizer returned unaligned token timestamps for contextual chunk")
	}

	units := tokens.Units(result.Tokens)
	kept := make([]int, 0, len(result.Tokens))
	keptUnits := make([]int, 0, len(units))
	for unitIndex, unit := range units {
		start := audioStart + float64(result.Timestamps[unit.Start])
		end := audioStart + resultTokenEnd(result, unit.End)
		midpoint := start + max(0, end-start)/2
		if midpoint < chunkStart || midpoint > chunkEnd || (!finalChunk && midpoint == chunkEnd) {
			continue
		}
		keptUnits = append(keptUnits, unitIndex)
		for index := unit.Start; index <= unit.End; index++ {
			kept = append(kept, index)
		}
	}

	mapped.Text = ""
	mapped.Tokens = make([]string, 0, len(kept))
	mapped.Timestamps = make([]float32, 0, len(kept))
	mapped.Durations = subsetTokenValues(result.Durations, result.Tokens, kept)
	mapped.LogProbs = subsetTokenValues(result.LogProbs, result.Tokens, kept)
	for _, index := range kept {
		mapped.Tokens = append(mapped.Tokens, result.Tokens[index])
		globalTimestamp := audioStart + float64(result.Timestamps[index])
		mapped.Timestamps = append(mapped.Timestamps, float32(max(0, globalTimestamp-chunkStart)))
	}
	rawWords := strings.Fields(result.Text)
	if len(rawWords) == len(units) {
		selectedWords := make([]string, 0, len(keptUnits))
		for _, index := range keptUnits {
			selectedWords = append(selectedWords, rawWords[index])
		}
		mapped.Text = strings.Join(selectedWords, " ")
	} else {
		mapped.Text = strings.TrimSpace(strings.Join(mapped.Tokens, ""))
	}
	return mapped, nil
}

func resultTokenEnd(result *asr.Result, index int) float64 {
	start := float64(result.Timestamps[index])
	end := start
	if len(result.Durations) == len(result.Tokens) {
		duration := float64(result.Durations[index])
		if duration > 0 && !math.IsNaN(duration) && !math.IsInf(duration, 0) {
			end += duration
		}
	}
	if index+1 < len(result.Timestamps) {
		next := float64(result.Timestamps[index+1])
		if end <= start || next < end {
			end = next
		}
	}
	return max(start, end)
}

func subsetTokenValues(values []float32, tokens []string, indices []int) []float32 {
	if len(values) == 0 || len(values) != len(tokens) {
		return nil
	}
	subset := make([]float32, 0, len(indices))
	for _, index := range indices {
		subset = append(subset, values[index])
	}
	return subset
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
