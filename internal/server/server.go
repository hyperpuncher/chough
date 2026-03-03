package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hyperpuncher/chough/internal/asr"
	"github.com/hyperpuncher/chough/internal/models"
	"github.com/hyperpuncher/chough/internal/output"
)

// Server is the HTTP server
type Server struct {
	httpServer *http.Server
	pool       RecognizerPool
	options    *ServerOptions
	version    string
	startTime  time.Time
}

// NewServer creates a new HTTP server
func NewServer(options *ServerOptions, pool RecognizerPool, version string) *Server {
	s := &Server{
		pool:      pool,
		options:   options,
		version:   version,
		startTime: time.Now(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/transcribe", s.handleTranscribe)
	mux.HandleFunc("/health", s.handleHealth)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", options.Host, options.Port),
		Handler: s.withMiddleware(mux),
	}

	return s
}

// Start starts the HTTP server
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Logging
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Fprintf(os.Stderr, "%s %s %s\n", r.Method, r.URL.Path, time.Since(start))
	})
}

func (s *Server) handleTranscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse request
	filePath, format, chunkSize, cleanup, err := s.parseRequest(r)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Create job
	job := &Job{
		ID:        strconv.FormatInt(time.Now().UnixNano(), 10),
		FilePath:  filePath,
		Format:    format,
		ChunkSize: chunkSize,
		Result:    make(chan JobResult, 1),
		Error:     make(chan error, 1),
		StartTime: time.Now(),
	}

	// Submit to pool
	if err := s.pool.Submit(job); err != nil {
		s.sendError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	// Wait for result
	select {
	case result := <-job.Result:
		s.sendFormattedResponse(w, format, result)
	case err := <-job.Error:
		s.sendError(w, http.StatusInternalServerError, err.Error())
	case <-time.After(10 * time.Minute):
		s.sendError(w, http.StatusRequestTimeout, "transcription timeout")
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	uptime := time.Since(s.startTime)
	s.sendJSON(w, http.StatusOK, HealthResponse{
		Status:      "healthy",
		ModelLoaded: true,
		Version:     s.version,
		Uptime:      uptime.Round(time.Second).String(),
		QueueSize:   s.pool.QueueSize(),
		Workers:     s.pool.TotalWorkers(),
		BusyWorkers: s.pool.BusyWorkers(),
	})
}

func (s *Server) parseRequest(r *http.Request) (filePath, format string, chunkSize int, cleanup func(), err error) {
	format = "text"
	chunkSize = 60

	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Handle file upload
		if err := r.ParseMultipartForm(s.options.MaxUploadMB * 1024 * 1024); err != nil {
			return "", "", 0, nil, fmt.Errorf("failed to parse multipart form: %w", err)
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			return "", "", 0, nil, fmt.Errorf("missing file field: %w", err)
		}
		defer file.Close()

		// Save to temp file
		tmpFile, err := os.CreateTemp("", "chough-upload-*-"+filepath.Base(header.Filename))
		if err != nil {
			return "", "", 0, nil, fmt.Errorf("failed to create temp file: %w", err)
		}

		if _, err := io.Copy(tmpFile, file); err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			return "", "", 0, nil, fmt.Errorf("failed to save file: %w", err)
		}
		tmpFile.Close()

		filePath = tmpFile.Name()
		cleanup = func() { os.Remove(filePath) }

		// Parse additional form fields
		if f := r.FormValue("format"); f != "" {
			format = strings.ToLower(f)
		}
		if c := r.FormValue("chunk_size"); c != "" {
			if n, err := strconv.Atoi(c); err == nil && n > 0 {
				chunkSize = n
			}
		}

	} else if strings.HasPrefix(contentType, "application/json") {
		// Handle JSON request (URL or base64)
		var req TranscribeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return "", "", 0, nil, fmt.Errorf("invalid JSON: %w", err)
		}

		if req.URL != "" {
			// Download from URL
			filePath, err = s.downloadFromURL(req.URL)
			if err != nil {
				return "", "", 0, nil, err
			}
			cleanup = func() { os.Remove(filePath) }

		} else if req.Base64 != "" {
			// Decode base64
			data, err := base64.StdEncoding.DecodeString(req.Base64)
			if err != nil {
				return "", "", 0, nil, fmt.Errorf("invalid base64: %w", err)
			}

			tmpFile, err := os.CreateTemp("", "chough-b64-*")
			if err != nil {
				return "", "", 0, nil, fmt.Errorf("failed to create temp file: %w", err)
			}

			if _, err := tmpFile.Write(data); err != nil {
				tmpFile.Close()
				os.Remove(tmpFile.Name())
				return "", "", 0, nil, fmt.Errorf("failed to write file: %w", err)
			}
			tmpFile.Close()

			filePath = tmpFile.Name()
			cleanup = func() { os.Remove(filePath) }

		} else {
			return "", "", 0, nil, fmt.Errorf("missing url or base64 in request")
		}

		if req.Format != "" {
			format = strings.ToLower(req.Format)
		}
		if req.ChunkSize > 0 {
			chunkSize = req.ChunkSize
		}

	} else {
		return "", "", 0, nil, fmt.Errorf("unsupported content type: %s", contentType)
	}

	// Validate format
	if format != "text" && format != "json" && format != "vtt" {
		if cleanup != nil {
			cleanup()
		}
		return "", "", 0, nil, fmt.Errorf("invalid format: %s (must be text, json, or vtt)", format)
	}

	return filePath, format, chunkSize, cleanup, nil
}

func (s *Server) downloadFromURL(url string) (string, error) {
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}

	// Check size
	if resp.ContentLength > s.options.MaxUploadMB*1024*1024 {
		return "", fmt.Errorf("file too large (max %d MB)", s.options.MaxUploadMB)
	}

	tmpFile, err := os.CreateTemp("", "chough-url-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	written, err := io.Copy(tmpFile, io.LimitReader(resp.Body, s.options.MaxUploadMB*1024*1024))
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to download: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to close file: %w", err)
	}

	// Validate minimum size
	if written < 44 { // Minimum WAV header size
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("downloaded file too small")
	}

	return tmpFile.Name(), nil
}

func (s *Server) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) sendError(w http.ResponseWriter, status int, message string) {
	s.sendJSON(w, status, TranscribeResponse{
		Success: false,
		Error:   message,
	})
}

func (s *Server) sendFormattedResponse(w http.ResponseWriter, format string, result JobResult) {
	switch format {
	case "text":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, result.Text)
	case "vtt":
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		output.WriteVTT(w, result.Chunks)
	default: // json
		s.sendJSON(w, http.StatusOK, TranscribeResponse{
			Success:        true,
			Duration:       result.Duration,
			ProcessingTime: result.ProcessingTime,
			RealtimeFactor: result.RealtimeFactor,
			Text:           result.Text,
			Chunks:         result.Chunks,
		})
	}
}

// LoadRecognizer loads the ASR recognizer
func LoadRecognizer() (*asr.Recognizer, error) {
	modelPath, err := models.GetModelPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	recognizer, err := asr.NewRecognizer(asr.DefaultConfig(modelPath))
	if err != nil {
		return nil, fmt.Errorf("failed to load model: %w", err)
	}

	return recognizer, nil
}
