package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/hyperpuncher/chough/internal/transcribe"
	"github.com/hyperpuncher/chough/internal/types"
	"github.com/hyperpuncher/chough/internal/worker"
)

type capturePool struct {
	jobs chan *worker.Job
}

func (p *capturePool) Submit(job *worker.Job) error {
	p.jobs <- job
	return nil
}

func (*capturePool) QueueSize() int    { return 0 }
func (*capturePool) BusyWorkers() int  { return 0 }
func (*capturePool) TotalWorkers() int { return 1 }

func TestHandleTranscribePropagatesRequestCancellation(t *testing.T) {
	pool := &capturePool{jobs: make(chan *worker.Job, 1)}
	server := NewServer(ServerOptions{MaxUploadMB: 1, Workers: 1, MaxQueueSize: 1}, pool, "test")
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
	if job.Cleanup != nil {
		job.Cleanup()
	}
}

func TestHandleTranscribeRejectsAtCapacityBeforeReadingBody(t *testing.T) {
	pool := &capturePool{jobs: make(chan *worker.Job, 1)}
	server := NewServer(ServerOptions{MaxUploadMB: 1, Workers: 1, MaxQueueSize: 1}, pool, "test")
	server.admission <- struct{}{}
	server.admission <- struct{}{}

	request := httptest.NewRequest(http.MethodPost, "/transcribe", strings.NewReader("not read"))
	response := httptest.NewRecorder()
	server.handleTranscribe(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if len(pool.jobs) != 0 {
		t.Fatal("request was submitted while server was at capacity")
	}
}

func TestParseRequestEnforcesMultipartFileLimit(t *testing.T) {
	server := &Server{options: ServerOptions{MaxUploadMB: 1}}
	for _, tt := range []struct {
		name    string
		size    int
		wantErr bool
	}{
		{name: "below limit", size: int(megabyte) - 1},
		{name: "at limit", size: int(megabyte)},
		{name: "above limit", size: int(megabyte) + 1, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, err := writer.CreateFormFile("file", "audio.wav")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := part.Write(bytes.Repeat([]byte{'a'}, tt.size)); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "/transcribe", &body)
			request.Header.Set("Content-Type", writer.FormDataContentType())
			_, _, _, cleanup, err := server.parseRequest(request)
			if tt.wantErr {
				if err == nil {
					cleanup()
					t.Fatal("parseRequest() error = nil, want size error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRequest() error = %v", err)
			}
			cleanup()
		})
	}
}

func TestParseRequestRejectsOptionsBeforeAcquiringAudio(t *testing.T) {
	requests := 0
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = w.Write(bytes.Repeat([]byte{'a'}, 44))
	}))
	defer source.Close()
	server := NewServer(ServerOptions{MaxUploadMB: 1, Workers: 1, AllowPrivateURLs: true}, nil, "test")

	for _, input := range []string{`"base64":"YQ=="`, `"url":"` + source.URL + `"`} {
		for _, option := range []string{`"format":"invalid"`, `"chunk_size":0`, `"chunk_size":301`} {
			t.Run(input+option, func(t *testing.T) {
				dir := t.TempDir()
				t.Setenv("TMPDIR", dir)
				request := httptest.NewRequest(http.MethodPost, "/transcribe", strings.NewReader("{"+input+","+option+"}"))
				request.Header.Set("Content-Type", "application/json")
				if _, _, _, _, err := server.parseRequest(request); err == nil {
					t.Fatal("expected invalid options to be rejected")
				}
				assertNoTempFiles(t, dir)
			})
		}
	}
	if requests != 0 {
		t.Fatalf("invalid options caused %d downloads", requests)
	}
}

func TestParseRequestRemovesMultipartSpillOnInvalidOptions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	server := &Server{options: ServerOptions{MaxUploadMB: 1}}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(bytes.Repeat([]byte{'a'}, 2*int(megabyte))); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("format", "invalid"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/transcribe", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if _, _, _, _, err := server.parseRequest(request); err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Fatalf("parseRequest() error = %v, want invalid format", err)
	}
	assertNoTempFiles(t, dir)
}

func TestCopyToTempRemovesPartialFiles(t *testing.T) {
	for _, test := range []struct {
		name   string
		reader io.Reader
	}{
		{name: "oversized", reader: strings.NewReader("too large")},
		{name: "read failure", reader: io.MultiReader(strings.NewReader("a"), iotest.ErrReader(errors.New("read failed")))},
		{name: "invalid base64", reader: base64.NewDecoder(base64.StdEncoding, strings.NewReader("YQ==!"))},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("TMPDIR", dir)
			if _, _, err := copyToTemp("chough-test-*", test.reader, 1); err == nil {
				t.Fatal("expected copy failure")
			}
			assertNoTempFiles(t, dir)
		})
	}
}

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary files remain: %v", entries)
	}
}

func TestParseRequestRejectsOversizedBase64(t *testing.T) {
	server := &Server{options: ServerOptions{MaxUploadMB: 1}}
	encoded := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{'a'}, int(megabyte)+1))
	body := `{"base64":"` + encoded + `"}`
	request := httptest.NewRequest(http.MethodPost, "/transcribe", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if _, _, _, _, err := server.parseRequest(request); !errors.Is(err, errUploadTooLarge) {
		t.Fatalf("parseRequest() error = %v, want errUploadTooLarge", err)
	}
}

func TestParseRequestValidatesJSONChunkSize(t *testing.T) {
	server := &Server{options: ServerOptions{MaxUploadMB: 1}}

	for _, body := range []string{
		`{"base64":"YQ==","chunk_size":0}`,
		`{"base64":"YQ==","chunk_size":-1}`,
		`{"base64":"YQ==","chunk_size":301}`,
	} {
		request := httptest.NewRequest("POST", "/transcribe", bytes.NewBufferString(body))
		request.Header.Set("Content-Type", "application/json")
		if _, _, _, _, err := server.parseRequest(request); err == nil {
			t.Fatalf("parseRequest(%s) error = nil, want chunk-size validation error", body)
		}
	}
}

func TestParseRequestUsesDefaultAndExplicitJSONChunkSize(t *testing.T) {
	server := &Server{options: ServerOptions{MaxUploadMB: 1}}

	for _, tt := range []struct {
		name string
		body string
		want int
	}{
		{name: "default", body: `{"base64":"YQ=="}`, want: 20},
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

func TestHandleTranscribeRejectsOversizedRequestBody(t *testing.T) {
	pool := &capturePool{jobs: make(chan *worker.Job, 1)}
	options := ServerOptions{MaxUploadMB: 1, Workers: 1, MaxQueueSize: 1}
	server := NewServer(options, pool, "test")
	body := `{"base64":"` + strings.Repeat("A", int(options.maxRequestBodyBytes("application/json")))
	request := httptest.NewRequest(http.MethodPost, "/transcribe", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.handleTranscribe(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestJSONResponseIncludesOptionalTokenData(t *testing.T) {
	server := &Server{}
	response := httptest.NewRecorder()
	if err := server.sendFormattedResponse(response, "json", transcribe.Result{
		Text: "hello",
		Chunks: []types.ChunkResult{{
			Text:      "hello",
			Tokens:    []string{"hello"},
			Durations: []float32{0.4},
			LogProbs:  []float32{-0.2},
		}},
	}); err != nil {
		t.Fatalf("sendFormattedResponse() error = %v", err)
	}

	var parsed TranscribeResponse
	if err := json.Unmarshal(response.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if len(parsed.Chunks) != 1 || len(parsed.Chunks[0].Durations) != 1 || len(parsed.Chunks[0].LogProbs) != 1 {
		t.Fatalf("optional token data missing: %+v", parsed.Chunks)
	}
}

func TestHTTPMethodAndHealthResponses(t *testing.T) {
	pool := &capturePool{jobs: make(chan *worker.Job, 1)}
	server := NewServer(ServerOptions{MaxUploadMB: 1, Workers: 1, MaxQueueSize: 1}, pool, "test")

	methodResponse := httptest.NewRecorder()
	server.handleTranscribe(methodResponse, httptest.NewRequest(http.MethodGet, "/transcribe", nil))
	if methodResponse.Code != http.StatusMethodNotAllowed {
		t.Fatalf("transcribe GET status = %d, want %d", methodResponse.Code, http.StatusMethodNotAllowed)
	}

	healthResponse := httptest.NewRecorder()
	server.handleHealth(healthResponse, httptest.NewRequest(http.MethodGet, "/health", nil))
	if healthResponse.Code != http.StatusOK || !strings.Contains(healthResponse.Body.String(), `"status":"healthy"`) {
		t.Fatalf("health response = %d %s", healthResponse.Code, healthResponse.Body.String())
	}
}
