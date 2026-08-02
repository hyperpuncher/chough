package server

import (
	"bytes"
	"context"
	"net/http/httptest"
	"testing"
	"time"
)

type capturePool struct {
	jobs chan *Job
}

func (p *capturePool) Submit(job *Job) error {
	p.jobs <- job
	return nil
}

func (*capturePool) QueueSize() int    { return 0 }
func (*capturePool) BusyWorkers() int  { return 0 }
func (*capturePool) TotalWorkers() int { return 1 }
func (*capturePool) Shutdown()         {}

func TestHandleTranscribePropagatesRequestCancellation(t *testing.T) {
	pool := &capturePool{jobs: make(chan *Job, 1)}
	server := NewServer(&ServerOptions{MaxUploadMB: 1}, pool, "test")
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest("POST", "/transcribe", bytes.NewBufferString(`{"base64":"YQ=="}`)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		server.handleTranscribe(response, request)
		close(done)
	}()

	job := <-pool.jobs
	cancel()
	select {
	case <-job.Context.Done():
	case <-time.After(time.Second):
		t.Fatal("job context was not canceled")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler did not return after cancellation")
	}
}

func TestParseRequestValidatesJSONChunkSize(t *testing.T) {
	server := &Server{options: &ServerOptions{MaxUploadMB: 1}}

	for _, body := range []string{
		`{"base64":"YQ==","chunk_size":0}`,
		`{"base64":"YQ==","chunk_size":-1}`,
	} {
		request := httptest.NewRequest("POST", "/transcribe", bytes.NewBufferString(body))
		request.Header.Set("Content-Type", "application/json")
		if _, _, _, _, err := server.parseRequest(request); err == nil {
			t.Fatalf("parseRequest(%s) error = nil, want chunk-size validation error", body)
		}
	}
}

func TestParseRequestUsesDefaultAndExplicitJSONChunkSize(t *testing.T) {
	server := &Server{options: &ServerOptions{MaxUploadMB: 1}}

	for _, tt := range []struct {
		name string
		body string
		want int
	}{
		{name: "default", body: `{"base64":"YQ=="}`, want: 60},
		{name: "explicit", body: `{"base64":"YQ==","chunk_size":30}`, want: 30},
	} {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest("POST", "/transcribe", bytes.NewBufferString(tt.body))
			request.Header.Set("Content-Type", "application/json")
			_, _, chunkSize, cleanup, err := server.parseRequest(request)
			if err != nil {
				t.Fatalf("parseRequest() error = %v", err)
			}
			defer cleanup()
			if chunkSize != tt.want {
				t.Fatalf("chunk size = %d, want %d", chunkSize, tt.want)
			}
		})
	}
}
