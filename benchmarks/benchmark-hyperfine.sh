#!/bin/bash
# Benchmark chough against whisper using hyperfine
# Usage: ./benchmark-hyperfine.sh <audio-file>

set -e

AUDIO_FILE="${1:-test/audio/audio-5min.wav}"
RESULTS_DIR="benchmarks/results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
mkdir -p "$RESULTS_DIR"

# Check requirements
if ! command -v hyperfine >/dev/null 2>&1; then
    echo "Error: hyperfine not installed"
    echo "Install: cargo install hyperfine  (or pacman -S hyperfine / brew install hyperfine)"
    exit 1
fi

if ! command -v ffprobe >/dev/null 2>&1; then
    echo "Error: ffprobe not found (install ffmpeg)"
    exit 1
fi

# Get audio duration
AUDIO_DURATION=$(ffprobe -v error -show_entries format=duration -of csv=p=0 "$AUDIO_FILE" 2>/dev/null | cut -d. -f1)
AUDIO_DURATION_FMT="$(($AUDIO_DURATION/60))m$(($AUDIO_DURATION%60))s"

echo "=========================================="
echo "Hyperfine Benchmark"
echo "File: $(basename "$AUDIO_FILE")"
echo "Duration: ${AUDIO_DURATION}s (${AUDIO_DURATION_FMT})"
echo "=========================================="
echo ""

# Build commands file for hyperfine
COMMANDS_FILE="$RESULTS_DIR/commands_$TIMESTAMP.txt"
echo "# Commands for hyperfine benchmark" > "$COMMANDS_FILE"
echo "# Generated: $(date)" >> "$COMMANDS_FILE"
echo "" >> "$COMMANDS_FILE"

# Check which tools are available
declare -a BENCHMARK_CMDS

# chough
if [ -f ./dist/chough ]; then
    BENCHMARK_CMDS+=("./dist/chough -f text '$AUDIO_FILE' 2>/dev/null")
    echo "✓ chough (dist/chough)"
fi

# whisper-ctranslate2
if command -v whisper-ctranslate2 >/dev/null 2>&1; then
    BENCHMARK_CMDS+=("whisper-ctranslate2 --model=medium --output_format=txt '$AUDIO_FILE' --output_dir='$RESULTS_DIR/whisper' 2>/dev/null")
    echo "✓ whisper-ctranslate2 (medium)"
elif command -v uvx >/dev/null 2>&1; then
    BENCHMARK_CMDS+=("uvx whisper-ctranslate2 --model=medium --output_format=txt '$AUDIO_FILE' --output_dir='$RESULTS_DIR/whisper' 2>/dev/null")
    echo "✓ whisper-ctranslate2 via uvx (medium)"
fi

# openai-whisper turbo via uvx
if command -v uvx >/dev/null 2>&1; then
    BENCHMARK_CMDS+=("uvx --from openai-whisper whisper --model=turbo --output_format=txt '$AUDIO_FILE' --output_dir='$RESULTS_DIR/whisper' 2>/dev/null")
    echo "✓ whisper (turbo via uvx)"
fi

if [ ${#BENCHMARK_CMDS[@]} -eq 0 ]; then
    echo "Error: No benchmark tools found!"
    echo "Build chough: just build"
    echo "Install whisper-ctranslate2: uv tool install whisper-ctranslate2"
    echo "Install whisper: uv tool install openai-whisper"
    exit 1
fi

echo ""
echo "Running hyperfine (this will take a few minutes)..."
echo ""

# Run hyperfine with memory tracking wrapper
RESULTS_JSON="$RESULTS_DIR/hyperfine_$TIMESTAMP.json"

hyperfine \
    --warmup 1 \
    --runs 3 \
    --export-json "$RESULTS_JSON" \
    --export-markdown "$RESULTS_DIR/hyperfine_$TIMESTAMP.md" \
    --style full \
    "${BENCHMARK_CMDS[@]}"

echo ""
echo "=========================================="
echo "Hyperfine results saved:"
echo "  JSON: $RESULTS_JSON"
echo "  Markdown: $RESULTS_DIR/hyperfine_$TIMESTAMP.md"
echo "=========================================="

# Now run memory benchmark separately with /usr/bin/time
echo ""
echo "Running memory benchmarks..."
MEMORY_FILE="$RESULTS_DIR/memory_$TIMESTAMP.txt"
echo "Memory Benchmark Results" > "$MEMORY_FILE"
echo "========================" >> "$MEMORY_FILE"
echo "File: $(basename "$AUDIO_FILE") (${AUDIO_DURATION}s)" >> "$MEMORY_FILE"
echo "Date: $(date)" >> "$MEMORY_FILE"
echo "" >> "$MEMORY_FILE"

for cmd in "${BENCHMARK_CMDS[@]}"; do
    tool_name=$(echo "$cmd" | awk '{print $1}' | sed 's|^.*/||')
    echo "Testing memory: $tool_name"
    
    if command -v /usr/bin/time >/dev/null 2>&1; then
        echo "" >> "$MEMORY_FILE"
        echo "--- $tool_name ---" >> "$MEMORY_FILE"
        { /usr/bin/time -v $cmd; } 2>> "$MEMORY_FILE" || true
    fi
done

# Generate summary
echo ""
echo "=========================================="
echo "Generating summary..."
echo "=========================================="

SUMMARY_FILE="$RESULTS_DIR/summary_$TIMESTAMP.md"
cat > "$SUMMARY_FILE" << EOF
# Benchmark Summary

**Audio:** $(basename "$AUDIO_FILE")  
**Duration:** ${AUDIO_DURATION}s (${AUDIO_DURATION_FMT})  
**Date:** $(date)

## Speed (Hyperfine)

See [hyperfine_$TIMESTAMP.md](hyperfine_$TIMESTAMP.md) for detailed results.

## Memory Usage

EOF

# Parse memory results
if [ -f "$MEMORY_FILE" ]; then
    grep -A5 "Maximum resident" "$MEMORY_FILE" 2>/dev/null | head -30 >> "$SUMMARY_FILE" || true
fi

cat >> "$SUMMARY_FILE" << EOF

## Realtime Factors

| Tool | Time (avg) | Realtime Factor |
|------|-----------|-----------------|
EOF

# Parse hyperfine JSON for realtime factors
if command -v jq >/dev/null 2>&1 && [ -f "$RESULTS_JSON" ]; then
    jq -r '.results[] | "| \(.command | split(" ")[0] | split("/")[-1]) | \(.mean | . * 1000 | round / 1000)s | \(.mean | '$AUDIO_DURATION' / . | round * 100 / 100)x |"' "$RESULTS_JSON" >> "$SUMMARY_FILE" 2>/dev/null || true
fi

cat >> "$SUMMARY_FILE" << EOF

Realtime Factor = Audio Duration / Processing Time (higher is better)

## System Info

\`\`\`
$(uname -a)
CPU: $(nproc) cores
Memory: $(free -h 2>/dev/null | grep Mem | awk '{print $2}' || echo "unknown")
\`\`\`

## Notes

- chough: Uses Parakeet TDT 0.6b V3 with 60s chunked processing
- whisper-ctranslate2: Faster-Whisper with CTranslate2 (medium model)
- whisper: OpenAI Whisper (turbo model)
EOF

echo ""
echo "Results:"
echo "  Summary: $SUMMARY_FILE"
echo "  Hyperfine JSON: $RESULTS_JSON"
echo "  Memory: $MEMORY_FILE"
echo ""

cat "$SUMMARY_FILE"
