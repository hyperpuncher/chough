package transcribe

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hyperpuncher/chough/internal/asr"
)

type fakeAudioProcessor struct {
	duration float64
	probe    func(context.Context) error
	extract  func(context.Context, float64, float64) error
	chunks   [][2]float64
}

func (f *fakeAudioProcessor) ProbeDuration(ctx context.Context, _ string) (float64, error) {
	if f.probe != nil {
		if err := f.probe(ctx); err != nil {
			return 0, err
		}
	}
	return f.duration, nil
}

func (f *fakeAudioProcessor) ExtractChunkWAV(ctx context.Context, _, _ string, start, duration float64) error {
	f.chunks = append(f.chunks, [2]float64{start, duration})
	if f.extract != nil {
		return f.extract(ctx, start, duration)
	}
	return nil
}

type fakeRecognizer struct {
	calls  int
	failAt int
	after  func(int)
}

func (f *fakeRecognizer) Transcribe(_ string) (*asr.Result, error) {
	f.calls++
	if f.after != nil {
		f.after(f.calls)
	}
	if f.calls == f.failAt {
		return nil, errors.New("recognizer failed")
	}
	return &asr.Result{
		Text:       fmt.Sprintf(" chunk %d ", f.calls),
		Timestamps: []float32{float32(f.calls)},
		Tokens:     []string{fmt.Sprintf("chunk%d", f.calls)},
	}, nil
}

func TestServiceTranscribe(t *testing.T) {
	processor := &fakeAudioProcessor{duration: 120.2}
	recognizer := &fakeRecognizer{}
	service := NewService(recognizer, processor)
	var completed []int

	result, err := service.Transcribe(context.Background(), "audio.mp3", Options{
		ChunkSize: 60,
		Progress: func(progress Progress) {
			completed = append(completed, progress.Completed)
		},
	})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if result.Duration != 120.2 {
		t.Fatalf("Duration = %v, want 120.2", result.Duration)
	}
	if len(result.Chunks) != 2 {
		t.Fatalf("len(Chunks) = %d, want 2", len(result.Chunks))
	}
	if result.Chunks[1].StartTime != 60 || result.Chunks[1].EndTime != 120.2 {
		t.Fatalf("final chunk = %+v, want 60-120.2", result.Chunks[1])
	}
	if result.Text != "chunk 1 chunk 2" {
		t.Fatalf("Text = %q, want %q", result.Text, "chunk 1 chunk 2")
	}
	if len(processor.chunks) != 2 || processor.chunks[1] != [2]float64{60, 60.2} {
		t.Fatalf("prepared chunks = %v", processor.chunks)
	}
	if got := fmt.Sprint(completed); got != "[0 1 2]" {
		t.Fatalf("progress completions = %s, want [0 1 2]", got)
	}
}

func TestServiceHandlesZeroDuration(t *testing.T) {
	processor := &fakeAudioProcessor{duration: 0}
	recognizer := &fakeRecognizer{}
	service := NewService(recognizer, processor)

	result, err := service.Transcribe(context.Background(), "audio.mp3", Options{ChunkSize: 60})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if result.Duration != 0 || len(result.Chunks) != 0 || result.Text != "" {
		t.Fatalf("unexpected zero-duration result: %+v", result)
	}
	if recognizer.calls != 0 || len(processor.chunks) != 0 {
		t.Fatalf("zero-duration calls: recognizer=%d chunks=%d", recognizer.calls, len(processor.chunks))
	}
}

func TestServiceReportsFailingChunk(t *testing.T) {
	service := NewService(&fakeRecognizer{failAt: 2}, &fakeAudioProcessor{duration: 120})

	_, err := service.Transcribe(context.Background(), "audio.mp3", Options{ChunkSize: 60})
	if err == nil {
		t.Fatal("Transcribe() error = nil, want recognizer error")
	}
	if !strings.Contains(err.Error(), "chunk 2 [60.000s-120.000s]") {
		t.Fatalf("Transcribe() error = %q, want chunk index and range", err)
	}
}

func TestServiceCancellationBeforeTranscription(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	processor := &fakeAudioProcessor{duration: 60}
	service := NewService(&fakeRecognizer{}, processor)

	_, err := service.Transcribe(ctx, "audio.mp3", Options{ChunkSize: 60})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Transcribe() error = %v, want context.Canceled", err)
	}
	if len(processor.chunks) != 0 {
		t.Fatalf("prepared %d chunks after cancellation", len(processor.chunks))
	}
}

func TestServiceCancellationDuringPreparation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	processor := &fakeAudioProcessor{
		duration: 60,
		extract: func(ctx context.Context, _, _ float64) error {
			cancel()
			<-ctx.Done()
			return ctx.Err()
		},
	}
	service := NewService(&fakeRecognizer{}, processor)

	_, err := service.Transcribe(ctx, "audio.mp3", Options{ChunkSize: 60})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Transcribe() error = %v, want context.Canceled", err)
	}
}

func TestServiceStopsBeforeNextChunkWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	recognizer := &fakeRecognizer{after: func(call int) {
		if call == 1 {
			cancel()
		}
	}}
	processor := &fakeAudioProcessor{duration: 120}
	service := NewService(recognizer, processor)

	_, err := service.Transcribe(ctx, "audio.mp3", Options{ChunkSize: 60})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Transcribe() error = %v, want context.Canceled", err)
	}
	if recognizer.calls != 1 || len(processor.chunks) != 1 {
		t.Fatalf("calls after cancellation: recognizer=%d chunks=%d", recognizer.calls, len(processor.chunks))
	}
}

func TestServiceRejectsInvalidChunkSize(t *testing.T) {
	service := NewService(&fakeRecognizer{}, &fakeAudioProcessor{duration: 60})
	if _, err := service.Transcribe(context.Background(), "audio.mp3", Options{}); err == nil {
		t.Fatal("Transcribe() error = nil, want chunk-size validation error")
	}
}
