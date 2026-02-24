package asr

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds ASR configuration
type Config struct {
	ModelPath   string
	NumThreads  int
	SampleRate  int
	FeatureDim  int
	Provider    string
	MaxSize     int64
	MaxDuration int
}

// ConfigFromEnv creates config from environment variables
func ConfigFromEnv() (*Config, error) {
	modelPath := os.Getenv("CHOUGH_MODEL")
	if modelPath == "" {
		return nil, fmt.Errorf("CHOUGH_MODEL environment variable is required")
	}

	// Check model files exist
	requiredFiles := []string{
		"encoder.int8.onnx",
		"decoder.int8.onnx",
		"joiner.int8.onnx",
		"tokens.txt",
	}

	for _, file := range requiredFiles {
		path := filepath.Join(modelPath, file)
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("model file not found: %s", path)
		}
	}

	threads := 4
	if v := os.Getenv("CHOUGH_THREADS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			threads = n
		}
	}

	maxSize := int64(100 * 1024 * 1024) // 100MB default
	if v := os.Getenv("CHOUGH_MAX_SIZE"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			maxSize = n
		}
	}

	maxDuration := 30 // 30 min default
	if v := os.Getenv("CHOUGH_MAX_DURATION"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxDuration = n
		}
	}

	return &Config{
		ModelPath:   modelPath,
		NumThreads:  threads,
		SampleRate:  16000,
		FeatureDim:  80,
		Provider:    "cpu",
		MaxSize:     maxSize,
		MaxDuration: maxDuration,
	}, nil
}
