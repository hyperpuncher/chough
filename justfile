# chough Justfile
# Fast chunked ASR CLI using sherpa-onnx

# Default recipe - show available commands
[private]
default:
    @just --list

# Build the CLI binary (current host)
build:
    go build -o dist/chough ./cmd/chough

# Validate goreleaser config
release-check:
    goreleaser check

# Local snapshot using goreleaser-cross container (publishing disabled)
release-cross-snapshot:
    docker run --rm \
      -v "$PWD":/work \
      -w /work \
      --entrypoint /bin/sh \
      ghcr.io/goreleaser/goreleaser-cross:latest \
      -lc 'apt-get update >/dev/null && apt-get install -y patchelf >/dev/null && go mod download && goreleaser release --snapshot --clean --skip=publish --skip=announce --skip=aur --skip=winget'

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
    @if [ -d dist ]; then trash dist; fi
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
