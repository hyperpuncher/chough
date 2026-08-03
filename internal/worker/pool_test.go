package worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hyperpuncher/chough/internal/asr"
	"github.com/hyperpuncher/chough/internal/transcribe"
)

type testAudioProcessor struct {
	duration float64
}

func (p testAudioProcessor) ProbeDuration(context.Context, string) (float64, error) {
	return p.duration, nil
}

func (testAudioProcessor) ExtractChunkWAV(context.Context, string, string, float64, float64) error {
	return nil
}

type testRecognizer struct {
	calls int
}

func (r *testRecognizer) Transcribe(string) (*asr.Result, error) {
	r.calls++
	return &asr.Result{Text: fmt.Sprintf("chunk %d", r.calls)}, nil
}

func TestPoolUsesSharedTranscriptionService(t *testing.T) {
	service := transcribe.NewService(&testRecognizer{}, testAudioProcessor{duration: 120})
	pool := NewPool(1, 1, service)
	defer pool.Shutdown()
	if err := pool.Submit(&Job{}); err == nil {
		t.Fatal("Submit() error = nil for unbuffered response channel")
	}

	cleaned := make(chan struct{})
	job := &Job{
		Context:   context.Background(),
		FilePath:  "audio.mp3",
		ChunkSize: 60,
		Cleanup:   func() { close(cleaned) },
		Response:  make(chan Response, 1),
	}
	if err := pool.Submit(job); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	select {
	case response := <-job.Response:
		if response.Error != nil {
			t.Fatalf("worker error = %v", response.Error)
		}
		result := response.Transcription
		if result.Duration != 120 || result.Text != "chunk 1 chunk 2" || len(result.Chunks) != 2 {
			t.Fatalf("unexpected worker result: %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for worker result")
	}
	select {
	case <-cleaned:
	case <-time.After(time.Second):
		t.Fatal("worker did not clean up accepted job")
	}
}
