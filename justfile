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

# Local archive snapshot using goreleaser-cross (publishing and Docker disabled)
release-cross-snapshot:
    docker run --rm \
      -v "$PWD":/work \
      -w /work \
      --entrypoint /bin/sh \
      ghcr.io/goreleaser/goreleaser-cross:latest \
      -lc 'apt-get update >/dev/null && apt-get install -y patchelf >/dev/null && git config --global --add safe.directory /work && go mod download && goreleaser release --snapshot --clean --skip=publish --skip=announce --skip=aur --skip=winget --skip=docker'

# Run the CLI with test file
dev *ARGS:
    go run ./cmd/chough {{ARGS}}

# Test with 30s chunks
test: build
    ./dist/chough -c 30 ./test/audio/audio-1min.wav

# Test different chunk sizes
test-chunks: build
    #!/usr/bin/env bash
    set -euo pipefail
    echo "=== 15s chunks ==="
    time ./dist/chough -c 15 ./test/audio/audio-1min.wav 2>&1 | tail -3
    echo ""
    echo "=== 30s chunks ==="
    time ./dist/chough -c 30 ./test/audio/audio-1min.wav 2>&1 | tail -3
    echo ""
    echo "=== 60s chunks ==="
    time ./dist/chough -c 60 ./test/audio/audio-1min.wav 2>&1 | tail -3

# Test 5-minute file (good stress test)
test-long: build
    time ./dist/chough -c 30 ./test/audio/audio-5min.wav

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

# Run all local validation checks
check:
    @files="$(gofmt -l .)"; if [ -n "$files" ]; then echo "unformatted Go files:"; echo "$files"; exit 1; fi
    go test -race ./...
    go vet ./...
    golangci-lint run ./...
    govulncheck ./...
    actionlint
    shellcheck benchmarks/*.sh scripts/*.sh
    git diff --check

# Show help
help:
    @just --list

# Benchmark cold CLI and warm server performance (requires hyperfine)
benchmark *ARGS:
    @command -v hyperfine >/dev/null 2>&1 || (echo "Install hyperfine: cargo install hyperfine" && exit 1)
    ./benchmarks/benchmark.sh {{ARGS}}
