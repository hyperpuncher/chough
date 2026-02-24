# chough Justfile
# Fast chunked ASR CLI using sherpa-onnx

# Default recipe - show available commands
[private]
default:
    @just --list

# Build the CLI binary
build:
    go build -o dist/chough ./cmd/chough

# Build for all platforms
build-all:
    #!/usr/bin/env bash
    set -e
    mkdir -p dist
    
    echo "Building for Linux x64..."
    GOOS=linux GOARCH=amd64 go build -o dist/chough-linux-x64 ./cmd/chough
    
    echo "Building for Linux arm64..."
    GOOS=linux GOARCH=arm64 go build -o dist/chough-linux-arm64 ./cmd/chough
    
    echo "Building for macOS x64..."
    GOOS=darwin GOARCH=amd64 go build -o dist/chough-macos-x64 ./cmd/chough
    
    echo "Building for macOS arm64..."
    GOOS=darwin GOARCH=arm64 go build -o dist/chough-macos-arm64 ./cmd/chough
    
    echo "Building for Windows x64..."
    GOOS=windows GOARCH=amd64 go build -o dist/chough-windows-x64.exe ./cmd/chough
    
    echo "Done! Binaries in dist/"

# Run the CLI with test file
dev *ARGS:
    CHOUGH_MODEL=./sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8 go run ./cmd/chough {{ARGS}}

# Test with default 30s chunks
test:
    CHOUGH_MODEL=./sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8 ./dist/chough ./test/audio/audio-1min.wav 30

# Test different chunk sizes
test-chunks:
    #!/usr/bin/env bash
    export CHOUGH_MODEL=./sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8
    echo "=== 15s chunks ==="
    time ./dist/chough ./test/audio/audio-1min.wav 15 2>&1 | tail -3
    echo ""
    echo "=== 30s chunks ==="
    time ./dist/chough ./test/audio/audio-1min.wav 30 2>&1 | tail -3
    echo ""
    echo "=== 60s chunks ==="
    time ./dist/chough ./test/audio/audio-1min.wav 60 2>&1 | tail -3

# Test 5-minute file (good stress test)
test-long:
    CHOUGH_MODEL=./sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8 time ./dist/chough ./test/audio/audio-5min.wav 30

# Run Go tests
unit-test:
    go test -v ./...

# Format code
fmt:
    go fmt ./...

# Run linter
lint:
    golangci-lint run ./...

# Clean build artifacts
clean:
    rm -rf dist/
    go clean -cache

# Download and tidy dependencies
get:
    go mod download
    go mod tidy

# Update dependencies
update:
    go get -u ./...
    go mod tidy

# Check for vulnerabilities
vuln:
    govulncheck ./...

# Show help
help:
    @just --list

# Benchmark against whisper (requires hyperfine)
benchmark AUDIO_FILE="test/audio/audio-1min.wav":
    @command -v hyperfine >/dev/null 2>&1 || (echo "Install hyperfine: cargo install hyperfine" && exit 1)
    @echo "Benchmarking with hyperfine..."
    ./benchmarks/benchmark.sh {{AUDIO_FILE}}

