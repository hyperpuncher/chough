package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func installModel(ctx context.Context, definition modelDefinition, modelDir string, client *http.Client, progress ProgressFunc) error {
	parent := filepath.Dir(modelDir)
	archive, err := os.CreateTemp(parent, ".chough-model-*.tar.bz2")
	if err != nil {
		return fmt.Errorf("create model download: %w", err)
	}
	archivePath := archive.Name()
	defer os.Remove(archivePath)

	if err := downloadArchive(ctx, definition, client, archive, progress); err != nil {
		return errors.Join(err, archive.Close())
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("close model download: %w", err)
	}

	installDir, err := os.MkdirTemp(parent, ".chough-model-install-*")
	if err != nil {
		return fmt.Errorf("create model installation directory: %w", err)
	}
	defer os.RemoveAll(installDir)

	reportProgress(progress, Progress{Stage: "extract"})
	if err := extractTarBz2(archivePath, installDir, definition.name); err != nil {
		return fmt.Errorf("extract model: %w", err)
	}
	if missing := missingModelFiles(installDir); len(missing) > 0 {
		return fmt.Errorf("downloaded model is incomplete; missing required files: %v", missing)
	}
	if err := os.Chmod(installDir, 0o755); err != nil {
		return fmt.Errorf("set model directory permissions: %w", err)
	}

	if err := os.Rename(installDir, modelDir); err != nil {
		if len(missingModelFiles(modelDir)) == 0 {
			return nil
		}
		return fmt.Errorf("install model: %w", err)
	}
	reportProgress(progress, Progress{Stage: "ready"})
	return nil
}

func downloadArchive(ctx context.Context, definition modelDefinition, client *http.Client, destination *os.File, progress ProgressFunc) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, definition.url, nil)
	if err != nil {
		return fmt.Errorf("create model download request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("download model: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download model: %s", response.Status)
	}
	if definition.size > 0 && response.ContentLength >= 0 && response.ContentLength != definition.size {
		return fmt.Errorf("download model: unexpected size %d; expected %d", response.ContentLength, definition.size)
	}

	total := response.ContentLength
	if total < 0 {
		total = definition.size
	}
	reporter := &downloadReporter{progress: progress, total: total}
	hash := sha256.New()
	written, err := io.CopyBuffer(io.MultiWriter(destination, hash, reporter), response.Body, make([]byte, 64*1024))
	if err != nil {
		return fmt.Errorf("download model body: %w", err)
	}
	if definition.size > 0 && written != definition.size {
		return fmt.Errorf("download model: unexpected size %d; expected %d", written, definition.size)
	}
	actualHash := hex.EncodeToString(hash.Sum(nil))
	if actualHash != definition.sha256 {
		return fmt.Errorf("download model checksum mismatch: got %s; expected %s", actualHash, definition.sha256)
	}
	reporter.finish(written)
	return nil
}

type downloadReporter struct {
	progress ProgressFunc
	total    int64
	written  int64
	reported int64
}

func (r *downloadReporter) Write(data []byte) (int, error) {
	r.written += int64(len(data))
	if r.written-r.reported >= 1024*1024 {
		r.reported = r.written
		reportProgress(r.progress, Progress{Stage: "download", Downloaded: r.written, Total: r.total})
	}
	return len(data), nil
}

func (r *downloadReporter) finish(written int64) {
	reportProgress(r.progress, Progress{Stage: "download", Downloaded: written, Total: r.total})
}

func reportProgress(progress ProgressFunc, event Progress) {
	if progress != nil {
		progress(event)
	}
}
