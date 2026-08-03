package server

import (
	"github.com/hyperpuncher/chough/internal/types"
	"github.com/hyperpuncher/chough/internal/worker"
)

type TranscribeRequest struct {
	URL       string `json:"url,omitempty"`
	Base64    string `json:"base64,omitempty"`
	Format    string `json:"format"`
	ChunkSize *int   `json:"chunk_size,omitempty"`
}

type TranscribeResponse struct {
	Success        bool                `json:"success"`
	Error          string              `json:"error,omitempty"`
	Duration       float64             `json:"duration_seconds"`
	ProcessingTime float64             `json:"processing_time_seconds"`
	RealtimeFactor float64             `json:"realtime_factor"`
	Text           string              `json:"text"`
	Chunks         []types.ChunkResult `json:"chunks,omitempty"`
}

type HealthResponse struct {
	Status      string `json:"status"`
	ModelLoaded bool   `json:"model_loaded"`
	Version     string `json:"version"`
	Uptime      string `json:"uptime"`
	QueueSize   int    `json:"queue_size"`
	Workers     int    `json:"workers"`
	BusyWorkers int    `json:"busy_workers"`
}

type ServerOptions struct {
	Host             string
	Port             int
	MaxUploadMB      int64
	Workers          int
	MaxQueueSize     int
	AllowPrivateURLs bool
}

func DefaultOptions() ServerOptions {
	return ServerOptions{
		Host:         "0.0.0.0",
		Port:         8080,
		MaxUploadMB:  1024,
		Workers:      2,
		MaxQueueSize: 10,
	}
}

// WorkerPool is the worker behavior needed by the HTTP server.
type WorkerPool interface {
	Submit(job *worker.Job) error
	QueueSize() int
	BusyWorkers() int
	TotalWorkers() int
}
