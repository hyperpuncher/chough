#!/bin/bash
# Benchmark chough against whisper using hyperfine
# Usage: ./benchmark.sh <audio-file>

AUDIO_FILE="${1:-test/audio/audio-1min.wav}"

if ! command -v hyperfine >/dev/null 2>&1; then
    echo "Error: hyperfine not installed (cargo install hyperfine)"
    exit 1
fi

AUDIO_DURATION=$(ffprobe -v error -show_entries format=duration -of csv=p=0 "$AUDIO_FILE" 2>/dev/null | cut -d. -f1)
DURATION_FMT="$(($AUDIO_DURATION / 60))m$(($AUDIO_DURATION % 60))s"

echo "Benchmark: $(basename "$AUDIO_FILE") (${AUDIO_DURATION}s / ${DURATION_FMT})"
echo ""

RESULTS_DIR="benchmarks/results"
mkdir -p "$RESULTS_DIR"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULTS_FILE="$RESULTS_DIR/benchmark_$TIMESTAMP.md"
JSON_FILE="$RESULTS_DIR/hyperfine_$TIMESTAMP.json"

# Run hyperfine with memory measurement
hyperfine \
    --warmup 1 \
    --runs 3 \
    --export-json "$JSON_FILE" \
    -n "chough" "./dist/chough -f text '$AUDIO_FILE' 2>/dev/null" \
    -n "whisper-ctranslate2" "uvx whisper-ctranslate2 --model=medium --output_format=txt '$AUDIO_FILE' --output_dir=/tmp 2>/dev/null" \
    -n "whisper" "uvx --from openai-whisper whisper --model=turbo --output_format=txt '$AUDIO_FILE' --output_dir=/tmp 2>/dev/null" \
    || true

# Generate markdown from JSON
if [ -f "$JSON_FILE" ]; then
    cat > "$RESULTS_FILE" << EOF
# Benchmark Results

**File:** $(basename "$AUDIO_FILE")  
**Duration:** ${AUDIO_DURATION}s (${DURATION_FMT})  
**Date:** $(date)

## Results

| Tool | Time | Realtime | Memory |
|------|------|----------|--------|
EOF

    # Parse JSON with jq
    jq -r --arg dur "$AUDIO_DURATION" '
        .results[] | 
        "| \(.command) | \(.mean | . * 100 | round / 100)s | \(.mean | ($dur | tonumber) / . | . * 10 | round / 10)x | \(.memory_usage_byte[0] / 1024 / 1024 | round)MB |"' \
        "$JSON_FILE" >> "$RESULTS_FILE"
fi

echo ""
echo "Results: $RESULTS_FILE"
echo "JSON: $JSON_FILE"
echo ""
cat "$RESULTS_FILE"
