package models

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

const (
	DefaultModelName = "sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8"
	ModelURL         = "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2"

	EncoderFile = "encoder.int8.onnx"
	DecoderFile = "decoder.int8.onnx"
	JoinerFile  = "joiner.int8.onnx"
	TokensFile  = "tokens.txt"
)

// Published by GitHub for the ModelURL release asset.
const defaultModelSHA256 = "1495e91b59d7fc53ddca91f3431758ec99f8fa253fdb0e1e851be0f72b14b0f9"

var requiredModelFiles = []string{EncoderFile, DecoderFile, JoinerFile, TokensFile}

type modelDefinition struct {
	name   string
	url    string
	sha256 string
	size   int64
}

var defaultModel = modelDefinition{
	name:   DefaultModelName,
	url:    ModelURL,
	sha256: defaultModelSHA256,
	size:   487211443,
}

// Progress describes model installation progress.
type Progress struct {
	Stage      string
	Downloaded int64
	Total      int64
}

// ProgressFunc receives model installation progress updates.
type ProgressFunc func(Progress)

// GetModelPath returns the configured model directory, downloading the default
// model when necessary.
func GetModelPath() (string, error) {
	return GetModelPathContext(context.Background(), nil)
}

// GetModelPathContext is GetModelPath with cancellation and progress support.
func GetModelPathContext(ctx context.Context, progress ProgressFunc) (string, error) {
	if configured := strings.TrimSpace(os.Getenv("CHOUGH_MODEL")); configured != "" {
		resolved, err := filepath.Abs(configured)
		if err != nil {
			return "", fmt.Errorf("resolve CHOUGH_MODEL %q: %w", configured, err)
		}
		if missing := missingModelFiles(resolved); len(missing) > 0 {
			return "", fmt.Errorf("CHOUGH_MODEL %q is invalid; missing required files: %s", resolved, strings.Join(missing, ", "))
		}
		return resolved, nil
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("find user cache directory: %w", err)
	}
	modelDir := filepath.Join(cacheDir, "chough", "models", defaultModel.name)
	return resolveModel(ctx, defaultModel, modelDir, newHTTPClient(), progress)
}

func resolveModel(ctx context.Context, definition modelDefinition, modelDir string, client *http.Client, progress ProgressFunc) (resolved string, retErr error) {
	if len(missingModelFiles(modelDir)) == 0 {
		return modelDir, nil
	}

	parent := filepath.Dir(modelDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("create model cache directory: %w", err)
	}
	lock := flock.New(modelDir + ".lock")
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return "", fmt.Errorf("lock model cache: %w", err)
	}
	if !locked {
		return "", ctx.Err()
	}
	defer func() { retErr = errors.Join(retErr, lock.Unlock()) }()

	if len(missingModelFiles(modelDir)) == 0 {
		return modelDir, nil
	}
	if err := os.RemoveAll(modelDir); err != nil {
		return "", fmt.Errorf("remove incomplete model: %w", err)
	}
	if err := installModel(ctx, definition, modelDir, client, progress); err != nil {
		return "", err
	}
	return modelDir, nil
}

func missingModelFiles(modelDir string) []string {
	missing := make([]string, 0, len(requiredModelFiles))
	for _, name := range requiredModelFiles {
		info, err := os.Stat(filepath.Join(modelDir, name))
		if err != nil || !info.Mode().IsRegular() {
			missing = append(missing, name)
		}
	}
	return missing
}

func newHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Client{
		Timeout: 30 * time.Minute,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			ResponseHeaderTimeout: time.Minute,
			TLSHandshakeTimeout:   30 * time.Second,
		},
	}
}
