package worker

import (
	"context"
	"testing"
	"time"

	"github.com/hyperpuncher/chough/internal/server"
	"github.com/hyperpuncher/chough/internal/transcribe"
)

type blockingAudioProcessor struct {
	started chan struct{}
}

func (p *blockingAudioProcessor) ProbeDuration(context.Context, string) (float64, error) {
	return 60, nil
}

func (p *blockingAudioProcessor) ExtractChunkWAV(ctx context.Context, _, _ string, _, _ float64) error {
	select {
	case p.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return ctx.Err()
}

func TestShutdownCancelsActiveAndQueuedJobs(t *testing.T) {
	processor := &blockingAudioProcessor{started: make(chan struct{}, 1)}
	service := transcribe.NewService(&testRecognizer{}, processor)
	pool := NewPool(1, 2, service)

	jobs := make([]*server.Job, 2)
	cleaned := make([]chan struct{}, 2)
	for i := range jobs {
		cleaned[i] = make(chan struct{})
		cleanup := cleaned[i]
		jobs[i] = &server.Job{
			Context:   context.Background(),
			FilePath:  "audio.mp3",
			ChunkSize: 60,
			Cleanup:   func() { close(cleanup) },
			Result:    make(chan server.JobResult, 1),
			Error:     make(chan error, 1),
		}
		if err := pool.Submit(jobs[i]); err != nil {
			t.Fatalf("Submit(job %d) error = %v", i, err)
		}
	}

	select {
	case <-processor.started:
	case <-time.After(time.Second):
		t.Fatal("active job did not start")
	}
	pool.Shutdown()

	for i, job := range jobs {
		select {
		case <-job.Error:
		case <-time.After(time.Second):
			t.Fatalf("job %d did not receive shutdown error", i)
		}
		select {
		case <-cleaned[i]:
		case <-time.After(time.Second):
			t.Fatalf("job %d was not cleaned up", i)
		}
	}
}
