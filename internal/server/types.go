package server

import (
	"time"

	"github.com/hyperpuncher/chough/internal/asr"
	"github.com/hyperpuncher/chough/internal/types"
)

// Job represents a transcription job
type Job struct {
	ID        string
	FilePath  string
	Format    string
	ChunkSize int
	Result    chan JobResult
	Error     chan error
	StartTime time.Time
}

// JobResult holds the result of a transcription job
type JobResult struct {
	Duration       float64
	ProcessingTime float64
	RealtimeFactor float64
	Text           string
	Chunks         []types.ChunkResult
}

// TranscribeRequest represents a transcription request
type TranscribeRequest struct {
	URL       string `json:"url,omitempty"`
	Base64    string `json:"base64,omitempty"`
	Format    string `json:"format"`     // text, json, vtt
	ChunkSize int    `json:"chunk_size"` // seconds
}

// TranscribeResponse represents a transcription response
type TranscribeResponse struct {
	Success        bool                `json:"success"`
	Error          string              `json:"error,omitempty"`
	Duration       float64             `json:"duration_seconds"`
	ProcessingTime float64             `json:"processing_time_seconds"`
	RealtimeFactor float64             `json:"realtime_factor"`
	Text           string              `json:"text"`
	Chunks         []types.ChunkResult `json:"chunks,omitempty"`
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status      string `json:"status"`
	ModelLoaded bool   `json:"model_loaded"`
	Version     string `json:"version"`
	Uptime      string `json:"uptime"`
	QueueSize   int    `json:"queue_size"`
	Workers     int    `json:"workers"`
	BusyWorkers int    `json:"busy_workers"`
}

// ServerOptions holds server configuration
type ServerOptions struct {
	Host         string
	Port         int
	MaxUploadMB  int64
	Workers      int
	MaxQueueSize int
}

// DefaultServerOptions returns default server options
func DefaultServerOptions() *ServerOptions {
	return &ServerOptions{
		Host:         "0.0.0.0",
		Port:         8080,
		MaxUploadMB:  1024,
		Workers:      2,
		MaxQueueSize: 10,
	}
}

// RecognizerPool is the interface for the worker pool
type RecognizerPool interface {
	Submit(job *Job) error
	QueueSize() int
	BusyWorkers() int
	TotalWorkers() int
	Shutdown()
}

// Recognizer is the interface for ASR recognizer
type Recognizer interface {
	Transcribe(audioPath string) (*asr.Result, error)
	Close()
}
