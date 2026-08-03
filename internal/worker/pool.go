package worker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/hyperpuncher/chough/internal/transcribe"
)

// Pool manages queued transcription work.
type Pool struct {
	workers   int
	queue     chan *Job
	service   *transcribe.Service
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.Mutex
	closed    bool
	busyCount atomic.Int32
}

func NewPool(workers, queueSize int, service *transcribe.Service) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	pool := &Pool{
		workers: workers,
		queue:   make(chan *Job, queueSize),
		service: service,
		ctx:     ctx,
		cancel:  cancel,
	}
	for range workers {
		pool.wg.Add(1)
		go pool.worker()
	}
	return pool
}

func (p *Pool) Submit(job *Job) error {
	if job == nil {
		return fmt.Errorf("job is required")
	}
	if cap(job.Response) < 1 {
		return fmt.Errorf("buffered job response channel is required")
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

func (p *Pool) QueueSize() int {
	return len(p.queue)
}

func (p *Pool) BusyWorkers() int {
	return int(p.busyCount.Load())
}

func (p *Pool) TotalWorkers() int {
	return p.workers
}

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
			p.finishJob(job, transcribe.Result{}, err)
			continue
		}
		if job.Context != nil {
			if err := job.Context.Err(); err != nil {
				p.finishJob(job, transcribe.Result{}, err)
				continue
			}
		}
		p.processJob(job)
	}
}

func (p *Pool) processJob(job *Job) {
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
		p.finishJob(job, transcribe.Result{}, fmt.Errorf("transcription failed: %w", err))
		return
	}
	p.finishJob(job, result, nil)
}

func (p *Pool) finishJob(job *Job, result transcribe.Result, err error) {
	if job.Cleanup != nil {
		job.Cleanup()
		job.Cleanup = nil
	}
	job.Response <- Response{Transcription: result, Error: err}
}
