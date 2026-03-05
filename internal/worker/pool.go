package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hyperpuncher/chough/internal/asr"
	"github.com/hyperpuncher/chough/internal/audio"
	"github.com/hyperpuncher/chough/internal/server"
	"github.com/hyperpuncher/chough/internal/types"
)

// Pool manages a pool of transcription workers
type Pool struct {
	workers    int
	queue      chan *server.Job
	recognizer *asr.Recognizer
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	busyCount  atomic.Int32
}

// NewPool creates a new worker pool
func NewPool(workers int, queueSize int, recognizer *asr.Recognizer) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		workers:    workers,
		queue:      make(chan *server.Job, queueSize),
		recognizer: recognizer,
		ctx:        ctx,
		cancel:     cancel,
	}

	// Start workers
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	return p
}

// Submit adds a job to the queue
func (p *Pool) Submit(job *server.Job) error {
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
	p.cancel()
	close(p.queue)
	p.wg.Wait()
}

func (p *Pool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case job, ok := <-p.queue:
			if !ok {
				return
			}
			p.processJob(job)
		}
	}
}

func (p *Pool) processJob(job *server.Job) {
	p.busyCount.Add(1)
	defer p.busyCount.Add(-1)

	// Clean up temp file after processing (the original input file)
	if job.FilePath != "" {
		defer os.Remove(job.FilePath)
	}

	startTime := time.Now()

	// Get audio duration
	duration, err := audio.ProbeDuration(job.FilePath)
	if err != nil {
		job.Error <- fmt.Errorf("failed to probe audio: %w", err)
		return
	}

	// Build boundaries for chunking
	boundaries := audio.BuildBoundaries(duration, job.ChunkSize)
	results := make([]types.ChunkResult, 0, len(boundaries)-1)

	// Process chunks
	for i := 0; i < len(boundaries)-1; i++ {
		chunkStart := boundaries[i]
		chunkEnd := boundaries[i+1]

		if chunkEnd-chunkStart < 0.5 {
			continue
		}

		result, err := p.transcribeChunk(job.FilePath, chunkStart, chunkEnd-chunkStart)
		if err != nil {
			job.Error <- fmt.Errorf("failed to transcribe chunk %d: %w", i+1, err)
			return
		}

		results = append(results, types.ChunkResult{
			StartTime:  chunkStart,
			EndTime:    chunkEnd,
			Text:       result.Text,
			Timestamps: result.Timestamps,
			Tokens:     result.Tokens,
		})
	}

	// Build full text
	fullText := ""
	for _, r := range results {
		if r.Text != "" {
			if fullText != "" {
				fullText += " "
			}
			fullText += r.Text
		}
	}

	processingTime := time.Since(startTime).Seconds()
	rtFactor := duration / processingTime
	if rtFactor < 0 {
		rtFactor = 0
	}

	job.Result <- server.JobResult{
		Duration:       duration,
		ProcessingTime: processingTime,
		RealtimeFactor: rtFactor,
		Text:           fullText,
		Chunks:         results,
	}
}

func (p *Pool) transcribeChunk(audioFile string, start, duration float64) (*asr.Result, error) {
	tmpDir, err := os.MkdirTemp("", "chough-chunk-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	chunkFile := filepath.Join(tmpDir, "chunk.wav")
	if err := audio.ExtractChunkWAV(audioFile, chunkFile, start, duration); err != nil {
		return nil, err
	}

	return p.recognizer.Transcribe(chunkFile)
}
