package worker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/hyperpuncher/chough/internal/server"
	"github.com/hyperpuncher/chough/internal/transcribe"
)

// Pool manages a pool of transcription workers
type Pool struct {
	workers   int
	queue     chan *server.Job
	service   *transcribe.Service
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.Mutex
	closed    bool
	busyCount atomic.Int32
}

// NewPool creates a new worker pool
func NewPool(workers int, queueSize int, service *transcribe.Service) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		workers: workers,
		queue:   make(chan *server.Job, queueSize),
		service: service,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Start workers
	for range workers {
		p.wg.Add(1)
		go p.worker()
	}

	return p
}

// Submit adds a job to the queue
func (p *Pool) Submit(job *server.Job) error {
	if job == nil {
		return fmt.Errorf("job is required")
	}
	if job.Context != nil {
		if err := job.Context.Err(); err != nil {
			return err
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return fmt.Errorf("worker pool is shutting down")
	}
	select {
	case p.queue <- job:
		return nil
	default:
		return fmt.Errorf("queue full (max %d jobs)", cap(p.queue))
	}
}

// QueueSize returns the current queue size
func (p *Pool) QueueSize() int {
	return len(p.queue)
}

// BusyWorkers returns the number of busy workers
func (p *Pool) BusyWorkers() int {
	return int(p.busyCount.Load())
}

// TotalWorkers returns the total number of workers
func (p *Pool) TotalWorkers() int {
	return p.workers
}

// Shutdown stops all workers
func (p *Pool) Shutdown() {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		p.cancel()
		close(p.queue)
	}
	p.mu.Unlock()
	p.wg.Wait()
}

func (p *Pool) worker() {
	defer p.wg.Done()

	for job := range p.queue {
		if err := p.ctx.Err(); err != nil {
			p.finishJob(job, server.JobResult{}, err)
			continue
		}
		if job.Context != nil {
			if err := job.Context.Err(); err != nil {
				p.finishJob(job, server.JobResult{}, err)
				continue
			}
		}
		p.processJob(job)
	}
}

func (p *Pool) processJob(job *server.Job) {
	p.busyCount.Add(1)
	defer p.busyCount.Add(-1)

	jobCtx := job.Context
	if jobCtx == nil {
		jobCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(jobCtx)
	stopPoolCancellation := context.AfterFunc(p.ctx, cancel)
	defer cancel()
	defer stopPoolCancellation()

	result, err := p.service.Transcribe(ctx, job.FilePath, transcribe.Options{ChunkSize: job.ChunkSize})
	if err != nil {
		p.finishJob(job, server.JobResult{}, fmt.Errorf("transcription failed: %w", err))
		return
	}

	rtFactor := 0.0
	if result.ProcessingTime > 0 && result.Duration > 0 {
		rtFactor = result.Duration / result.ProcessingTime.Seconds()
	}
	p.finishJob(job, server.JobResult{
		Duration:       result.Duration,
		ProcessingTime: result.ProcessingTime.Seconds(),
		RealtimeFactor: rtFactor,
		Text:           result.Text,
		Chunks:         result.Chunks,
	}, nil)
}

func (p *Pool) finishJob(job *server.Job, result server.JobResult, err error) {
	if job.Cleanup != nil {
		job.Cleanup()
		job.Cleanup = nil
	}
	if err != nil {
		job.Error <- err
		return
	}
	job.Result <- result
}
