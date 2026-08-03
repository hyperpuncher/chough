package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/hyperpuncher/chough/internal/types"
)

type remoteJSONResponse struct {
	Success  bool                `json:"success"`
	Error    string              `json:"error,omitempty"`
	Duration float64             `json:"duration_seconds"`
	Text     string              `json:"text"`
	Chunks   []types.ChunkResult `json:"chunks,omitempty"`
}

func resolveRemoteURL() (string, error) {
	raw := strings.TrimSpace(os.Getenv("CHOUGH_URL"))
	if raw == "" {
		return "", fmt.Errorf("--remote requires CHOUGH_URL (e.g. CHOUGH_URL=http://localhost:8080)")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid CHOUGH_URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid CHOUGH_URL %q: must start with http:// or https://", raw)
	}
	if u.Host == "" {
		return "", fmt.Errorf("invalid CHOUGH_URL %q: missing host", raw)
	}

	return strings.TrimRight(raw, "/"), nil
}

func transcribeRemote(ctx context.Context, serverURL, audioFile string, chunkSize int) ([]types.ChunkResult, float64, error) {
	file, err := os.Open(audioFile)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open audio file: %w", err)
	}

	bodyReader, bodyWriter := io.Pipe()
	writer := multipart.NewWriter(bodyWriter)
	endpoint := serverURL + "/transcribe"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bodyReader)
	if err != nil {
		cleanupErr := errors.Join(file.Close(), bodyReader.Close(), bodyWriter.Close())
		return nil, 0, errors.Join(fmt.Errorf("failed to create request: %w", err), cleanupErr)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())

	go func() {
		defer file.Close()
		err := writeMultipartUpload(writer, file, filepath.Base(audioFile), chunkSize)
		_ = bodyWriter.CloseWithError(err)
	}()
	defer func() { _ = bodyReader.Close() }()

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, 0, fmt.Errorf("remote request failed to %s: %w", endpoint, err)
	}
	defer response.Body.Close()

	var parsed remoteJSONResponse
	if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
		return nil, 0, fmt.Errorf("failed to decode remote JSON response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		if parsed.Error != "" {
			return nil, 0, fmt.Errorf("remote server returned %s: %s", response.Status, parsed.Error)
		}
		return nil, 0, fmt.Errorf("remote server returned %s", response.Status)
	}

	if !parsed.Success {
		if parsed.Error != "" {
			return nil, 0, fmt.Errorf("remote transcription failed: %s", parsed.Error)
		}
		return nil, 0, fmt.Errorf("remote transcription failed")
	}

	if len(parsed.Chunks) > 0 {
		return parsed.Chunks, parsed.Duration, nil
	}
	if strings.TrimSpace(parsed.Text) == "" {
		return []types.ChunkResult{}, parsed.Duration, nil
	}
	return []types.ChunkResult{{Text: parsed.Text}}, parsed.Duration, nil
}

func writeMultipartUpload(writer *multipart.Writer, file io.Reader, filename string, chunkSize int) error {
	fileWriter, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return fmt.Errorf("create multipart file field: %w", err)
	}
	if _, err := io.Copy(fileWriter, file); err != nil {
		return fmt.Errorf("stream audio file: %w", err)
	}
	if err := writer.WriteField("format", "json"); err != nil {
		return fmt.Errorf("set format: %w", err)
	}
	if err := writer.WriteField("chunk_size", fmt.Sprintf("%d", chunkSize)); err != nil {
		return fmt.Errorf("set chunk_size: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finalize multipart body: %w", err)
	}
	return nil
}
