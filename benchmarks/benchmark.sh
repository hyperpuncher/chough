#!/usr/bin/env bash
# Benchmark chough against other implementations or another chough build.
# Usage:
#   ./benchmarks/benchmark.sh <audio-file>
#   ./benchmarks/benchmark.sh <audio-file> <baseline-binary> [candidate-binary]

set -euo pipefail

AUDIO_FILE="${1:-test/audio/audio-1min.wav}"
BASELINE_BINARY="${2:-}"
CANDIDATE_BINARY="${3:-./dist/chough}"
BASELINE_NAME="${BASELINE_NAME:-baseline}"
CANDIDATE_NAME="${CANDIDATE_NAME:-candidate}"
RUNS="${RUNS:-3}"

if ! command -v hyperfine >/dev/null 2>&1; then
	echo "Error: hyperfine not installed (cargo install hyperfine)"
	exit 1
fi

AUDIO_DURATION=$(ffprobe -v error -show_entries format=duration -of csv=p=0 "$AUDIO_FILE" 2>/dev/null | cut -d. -f1)
DURATION_FMT="$((AUDIO_DURATION / 60))m$((AUDIO_DURATION % 60))s"

echo "Benchmark: $(basename "$AUDIO_FILE") (${AUDIO_DURATION}s / ${DURATION_FMT})"
echo ""

RESULTS_DIR="benchmarks/results"
mkdir -p "$RESULTS_DIR"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULTS_FILE="$RESULTS_DIR/benchmark_$TIMESTAMP.md"
JSON_FILE="$RESULTS_DIR/hyperfine_$TIMESTAMP.json"

CHOUGH_CMD="'$CANDIDATE_BINARY' -f text '$AUDIO_FILE' >/dev/null 2>/dev/null"

# Run hyperfine with memory measurement.
if [ -n "$BASELINE_BINARY" ]; then
	BASELINE_CMD="'$BASELINE_BINARY' -f text '$AUDIO_FILE' >/dev/null 2>/dev/null"
	hyperfine \
		--warmup 1 \
		--runs "$RUNS" \
		--export-json "$JSON_FILE" \
		-n "$BASELINE_NAME" "$BASELINE_CMD" \
		-n "$CANDIDATE_NAME" "$CHOUGH_CMD"
else
	CHOUGH_REMOTE_CMD="'$CANDIDATE_BINARY' --remote -f text '$AUDIO_FILE' >/dev/null 2>/dev/null"
	PARAKEET_CMD="/tmp/parakeet.cpp/build/examples/cli/parakeet-cli transcribe --model /tmp/parakeet.cpp/models/tdt-0.6b-v3-q8_0.gguf --input '$AUDIO_FILE' --threads 4 >/dev/null 2>/dev/null"
	hyperfine \
		--warmup 1 \
		--runs "$RUNS" \
		--export-json "$JSON_FILE" \
		-n "chough" "$CHOUGH_CMD" \
		-n "chough (remote)" "$CHOUGH_REMOTE_CMD" \
		-n "parakeet.cpp" "$PARAKEET_CMD"
fi

# Generate markdown from JSON
if [ -f "$JSON_FILE" ]; then
	cat >"$RESULTS_FILE" <<EOF
# Benchmark Results

- **File:** $(basename "$AUDIO_FILE")
- **Duration:** ${AUDIO_DURATION}s (${DURATION_FMT})
- **Date:** $(date)
- **CPU:** $(lscpu 2>/dev/null | awk -F: '/Model name/{print $2}' | xargs)
- **Runs:** $RUNS (+ 1 warmup)

## Results

| Tool | Mean | Stddev | Realtime | Peak memory |
|------|------|--------|----------|-------------|
EOF

	jq -r --arg dur "$AUDIO_DURATION" '
        .results[] |
        "| \(.command) | \(.mean * 1000 | round)ms | \(.stddev * 1000 | round)ms | \(($dur | tonumber) / .mean * 10 | round / 10)x | \(.memory_usage_byte | max / 1024 / 1024 | round)MB |"' \
		"$JSON_FILE" >>"$RESULTS_FILE"
fi

echo ""
echo "Results: $RESULTS_FILE"
echo "JSON: $JSON_FILE"
echo ""
bunx oxfmt "$RESULTS_FILE" 2>/dev/null
cat "$RESULTS_FILE"
