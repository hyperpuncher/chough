package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hyperpuncher/chough/internal/output"
	"github.com/hyperpuncher/chough/internal/transcribe"
	"github.com/hyperpuncher/chough/internal/worker"
)

var errUploadTooLarge = errors.New("upload exceeds configured limit")

// Server is the HTTP server
type Server struct {
	httpServer     *http.Server
	pool           WorkerPool
	options        ServerOptions
	version        string
	startTime      time.Time
	urlPolicy      *urlPolicy
	downloadClient *http.Client
	admission      chan struct{}
}

// NewServer creates a new HTTP server
func NewServer(options ServerOptions, pool WorkerPool, version string) *Server {
	policy := newURLPolicy(options.AllowPrivateURLs, nil)
	s := &Server{
		pool:           pool,
		options:        options,
		version:        version,
		startTime:      time.Now(),
		urlPolicy:      policy,
		downloadClient: newDownloadClient(policy),
		admission:      make(chan struct{}, options.Workers+options.MaxQueueSize),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/transcribe", s.handleTranscribe)
	mux.HandleFunc("/health", s.handleHealth)

	s.httpServer = &http.Server{
		Addr:              net.JoinHostPort(options.Host, strconv.Itoa(options.Port)),
		Handler:           s.withMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}

	return s
}

// Start starts the HTTP server
func (s *Server) Start() error {
	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

type responseLogger struct {
	http.ResponseWriter
	status int
}

func (w *responseLogger) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseLogger) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (s *Server) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logged := &responseLogger{ResponseWriter: w}
		started := time.Now()
		defer func() {
			status := logged.status
			if status == 0 {
				status = http.StatusOK
			}
			fmt.Fprintf(os.Stderr, "%s %s status=%d remote=%s duration=%s queue=%d busy=%d\n",
				r.Method, r.URL.Path, status, r.RemoteAddr, time.Since(started), s.pool.QueueSize(), s.pool.BusyWorkers())
		}()

		next.ServeHTTP(logged, r)
	})
}

func (s *Server) handleTranscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	select {
	case s.admission <- struct{}{}:
		defer func() { <-s.admission }()
	default:
		s.sendError(w, http.StatusServiceUnavailable, "server is at capacity")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.options.maxRequestBodyBytes(r.Header.Get("Content-Type")))

	filePath, format, chunkSize, cleanup, err := s.parseRequest(r)
	if err != nil {
		status := http.StatusBadRequest
		var maxBytesError *http.MaxBytesError
		if errors.Is(err, errUploadTooLarge) || errors.As(err, &maxBytesError) {
			status = http.StatusRequestEntityTooLarge
		}
		s.sendError(w, status, err.Error())
		return
	}

	jobCtx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	job := &worker.Job{
		Context:   jobCtx,
		FilePath:  filePath,
		ChunkSize: chunkSize,
		Cleanup:   cleanup,
		Response:  make(chan worker.Response, 1),
	}

	if err := s.pool.Submit(job); err != nil {
		cleanup()
		s.sendError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	select {
	case response := <-job.Response:
		if response.Error != nil {
			s.sendJobError(w, response.Error)
			return
		}
		if err := s.sendFormattedResponse(w, format, response.Transcription); err != nil {
			fmt.Fprintf(os.Stderr, "write transcription response: %v\n", err)
		}
	case <-jobCtx.Done():
		if jobCtx.Err() == context.DeadlineExceeded {
			s.sendError(w, http.StatusRequestTimeout, "transcription timeout")
		}
	}
}

func (s *Server) sendJobError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, context.Canceled):
		return
	case errors.Is(err, context.DeadlineExceeded):
		s.sendError(w, http.StatusRequestTimeout, "transcription timeout")
	default:
		s.sendError(w, http.StatusInternalServerError, err.Error())
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
	chunkSize = transcribe.DefaultChunkSize
	defer func() {
		if err != nil && cleanup != nil {
			cleanup()
			cleanup = nil
		}
	}()

	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		// ParseMultipartForm spills upload data beyond 32 MB to disk.
		multipartMemory := min(s.options.maxUploadBytes(), 32*megabyte)
		if err := r.ParseMultipartForm(multipartMemory); err != nil {
			return "", "", 0, nil, fmt.Errorf("failed to parse multipart form: %w", err)
		}
		defer r.MultipartForm.RemoveAll()

		file, header, err := r.FormFile("file")
		if err != nil {
			return "", "", 0, nil, fmt.Errorf("missing file field: %w", err)
		}
		defer file.Close()
		if header.Size > s.options.maxUploadBytes() {
			return "", "", 0, nil, fmt.Errorf("%w: maximum is %d MB", errUploadTooLarge, s.options.MaxUploadMB)
		}

		filePath, _, err = copyToTemp("chough-upload-*", file, s.options.maxUploadBytes())
		if err != nil {
			return "", "", 0, nil, fmt.Errorf("save uploaded file: %w", err)
		}
		cleanup = func() { os.Remove(filePath) }

		if f := r.FormValue("format"); f != "" {
			format = strings.ToLower(f)
		}
		if c := r.FormValue("chunk_size"); c != "" {
			n, parseErr := strconv.Atoi(c)
			if parseErr != nil || n <= 0 {
				return "", "", 0, nil, fmt.Errorf("chunk_size must be a positive integer")
			}
			chunkSize = n
		}

	} else if strings.HasPrefix(contentType, "application/json") {
		var req TranscribeRequest
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&req); err != nil {
			return "", "", 0, nil, fmt.Errorf("invalid JSON: %w", err)
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			return "", "", 0, nil, fmt.Errorf("invalid JSON: expected one object")
		}

		if req.URL != "" {
			filePath, err = s.downloadFromURL(r.Context(), req.URL)
			if err != nil {
				return "", "", 0, nil, err
			}
			cleanup = func() { os.Remove(filePath) }

		} else if req.Base64 != "" {
			maxEncoded := (s.options.maxUploadBytes() + 2) / 3 * 4
			if int64(len(req.Base64)) > maxEncoded {
				return "", "", 0, nil, fmt.Errorf("%w: maximum is %d MB", errUploadTooLarge, s.options.MaxUploadMB)
			}
			decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(req.Base64))
			filePath, _, err = copyToTemp("chough-b64-*", decoder, s.options.maxUploadBytes())
			if err != nil {
				if errors.Is(err, errUploadTooLarge) {
					return "", "", 0, nil, err
				}
				return "", "", 0, nil, fmt.Errorf("invalid base64: %w", err)
			}
			cleanup = func() { os.Remove(filePath) }

		} else {
			return "", "", 0, nil, fmt.Errorf("missing url or base64 in request")
		}

		if req.Format != "" {
			format = strings.ToLower(req.Format)
		}
		if req.ChunkSize != nil {
			if *req.ChunkSize <= 0 {
				return "", "", 0, nil, fmt.Errorf("chunk_size must be positive")
			}
			chunkSize = *req.ChunkSize
		}

	} else {
		return "", "", 0, nil, fmt.Errorf("unsupported content type: %s", contentType)
	}

	if format != "text" && format != "json" && format != "vtt" {
		return "", "", 0, nil, fmt.Errorf("invalid format: %s (must be text, json, or vtt)", format)
	}
	if err := transcribe.ValidateChunkSize(chunkSize); err != nil {
		return "", "", 0, nil, err
	}

	return filePath, format, chunkSize, cleanup, nil
}

func (s *Server) downloadFromURL(ctx context.Context, rawURL string) (string, error) {
	parsed, err := s.urlPolicy.validate(ctx, rawURL)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", fmt.Errorf("create download request: %w", err)
	}
	resp, err := s.downloadClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}
	if resp.ContentLength > s.options.maxUploadBytes() {
		return "", fmt.Errorf("%w: maximum is %d MB", errUploadTooLarge, s.options.MaxUploadMB)
	}

	filePath, written, err := copyToTemp("chough-url-*", resp.Body, s.options.maxUploadBytes())
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	if written < 44 {
		os.Remove(filePath)
		return "", fmt.Errorf("downloaded file too small")
	}
	return filePath, nil
}

func copyToTemp(pattern string, source io.Reader, maxBytes int64) (path string, written int64, err error) {
	tmpFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", 0, fmt.Errorf("create temporary file: %w", err)
	}
	path = tmpFile.Name()
	defer func() {
		if err != nil {
			tmpFile.Close()
			os.Remove(path)
		}
	}()

	written, err = io.Copy(tmpFile, io.LimitReader(source, maxBytes+1))
	if err != nil {
		return "", 0, err
	}
	if written > maxBytes {
		return "", 0, errUploadTooLarge
	}
	if err = tmpFile.Close(); err != nil {
		return "", 0, fmt.Errorf("close temporary file: %w", err)
	}
	return path, written, nil
}

func (s *Server) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	_ = writeJSON(w, status, data)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

func (s *Server) sendError(w http.ResponseWriter, status int, message string) {
	s.sendJSON(w, status, TranscribeResponse{
		Success: false,
		Error:   message,
	})
}

func (s *Server) sendFormattedResponse(w http.ResponseWriter, format string, result transcribe.Result) error {
	realtimeFactor := 0.0
	if result.ProcessingTime > 0 && result.Duration > 0 {
		realtimeFactor = result.Duration / result.ProcessingTime.Seconds()
	}

	switch format {
	case "text":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, err := fmt.Fprintln(w, result.Text)
		return err
	case "vtt":
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		return output.WriteVTT(w, result.Chunks)
	case "json":
		return writeJSON(w, http.StatusOK, TranscribeResponse{
			Success:        true,
			Duration:       result.Duration,
			ProcessingTime: result.ProcessingTime.Seconds(),
			RealtimeFactor: realtimeFactor,
			Text:           result.Text,
			Chunks:         result.Chunks,
		})
	default:
		return fmt.Errorf("unsupported response format %q", format)
	}
}
