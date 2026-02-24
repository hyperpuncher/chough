#!/bin/bash
# Benchmark chough against whisper implementations
# Usage: ./benchmark.sh <audio-file>

set -e

AUDIO_FILE="${1:-test/audio/audio-5min.wav}"
RESULTS_DIR="benchmarks/results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULTS_FILE="$RESULTS_DIR/benchmark_$TIMESTAMP.md"

mkdir -p "$RESULTS_DIR"

# Check audio file exists
if [ ! -f "$AUDIO_FILE" ]; then
    echo "Error: Audio file not found: $AUDIO_FILE"
    echo "Usage: $0 <audio-file>"
    exit 1
fi

# Get audio duration using ffprobe
AUDIO_DURATION=$(ffprobe -v error -show_entries format=duration -of csv=p=0 "$AUDIO_FILE" 2>/dev/null | cut -d. -f1)
if [ -z "$AUDIO_DURATION" ]; then
    echo "Error: Could not determine audio duration. Is ffmpeg installed?"
    exit 1
fi

echo "=========================================="
echo "Benchmark: $(basename "$AUDIO_FILE")"
echo "Duration: ${AUDIO_DURATION}s (~$(($AUDIO_DURATION/60))min)"
echo "=========================================="
echo ""

# Function to run benchmark with detailed stats
run_benchmark() {
    local name="$1"
    local cmd="$2"
    local output_file="$RESULTS_DIR/${name}_${TIMESTAMP}.txt"
    
    echo "Testing: $name"
    echo "Command: $cmd"
    
    # Run with /usr/bin/time for detailed stats
    # -v gives max memory, cpu %, etc.
    if command -v /usr/bin/time >/dev/null 2>&1; then
        # GNU time available
        /usr/bin/time -v -o "$output_file.stats" bash -c "$cmd" > "$output_file" 2>&1 || true
        
        # Extract stats
        WALL_TIME=$(grep "Elapsed (wall clock) time" "$output_file.stats" | awk '{print $8}')
        USER_TIME=$(grep "User time (seconds)" "$output_file.stats" | awk '{print $4}')
        SYS_TIME=$(grep "System time (seconds)" "$output_file.stats" | awk '{print $4}')
        MAX_MEMORY_KB=$(grep "Maximum resident set size" "$output_file.stats" | awk '{print $6}')
        CPU_PERCENT=$(grep "Percent of CPU" "$output_file.stats" | awk '{print $3}' | tr -d '%')
    else
        # Fallback to bash time
        { time bash -c "$cmd"; } > "$output_file" 2>&1 || true
        WALL_TIME="N/A"
        MAX_MEMORY_KB="N/A"
        CPU_PERCENT="N/A"
    fi
    
    # Calculate realtime factor
    if [ "$WALL_TIME" != "N/A" ]; then
        # Parse time format (e.g., 0:14.50 or 14.50)
        if [[ "$WALL_TIME" == *:* ]]; then
            MINUTES=$(echo "$WALL_TIME" | cut -d: -f1)
            SECONDS=$(echo "$WALL_TIME" | cut -d: -f2)
            WALL_SECONDS=$(echo "$MINUTES * 60 + $SECONDS" | bc 2>/dev/null || echo "0")
        else
            WALL_SECONDS=$WALL_TIME
        fi
        
        if [ -n "$WALL_SECONDS" ] && [ "$WALL_SECONDS" != "0" ]; then
            REALTIME_FACTOR=$(echo "scale=1; $AUDIO_DURATION / $WALL_SECONDS" | bc 2>/dev/null || echo "N/A")
        else
            REALTIME_FACTOR="N/A"
        fi
    else
        REALTIME_FACTOR="N/A"
    fi
    
    # Convert memory to MB
    if [ "$MAX_MEMORY_KB" != "N/A" ] && [ -n "$MAX_MEMORY_KB" ]; then
        MAX_MEMORY_MB=$((MAX_MEMORY_KB / 1024))
    else
        MAX_MEMORY_MB="N/A"
    fi
    
    echo "  Wall time: ${WALL_TIME}s"
    echo "  Realtime factor: ${REALTIME_FACTOR}x"
    echo "  Max memory: ${MAX_MEMORY_MB}MB"
    echo "  CPU usage: ${CPU_PERCENT}%"
    echo ""
    
    # Append to results
    echo "| $name | ${WALL_TIME}s | ${REALTIME_FACTOR}x | ${MAX_MEMORY_MB}MB | ${CPU_PERCENT}% |" >> "$RESULTS_FILE.tmp"
}

echo "# Benchmark Results" > "$RESULTS_FILE"
echo "" >> "$RESULTS_FILE"
echo "**File:** $(basename "$AUDIO_FILE")  " >> "$RESULTS_FILE"
echo "**Duration:** ${AUDIO_DURATION}s (~$(($AUDIO_DURATION/60))min)  " >> "$RESULTS_FILE"
echo "**Date:** $(date)  " >> "$RESULTS_FILE"
echo "" >> "$RESULTS_FILE"
echo "| Tool | Time | Realtime | Memory | CPU |" >> "$RESULTS_FILE"
echo "|------|------|----------|--------|-----|" >> "$RESULTS_FILE"

# Benchmark chough
if command -v chough >/dev/null 2>&1 || [ -f ./dist/chough ]; then
    CHOUGH_CMD="./dist/chough -f text '$AUDIO_FILE' 2>/dev/null"
    run_benchmark "chough" "$CHOUGH_CMD"
else
    echo "chough not found, skipping..."
fi

# Benchmark whisper-ctranslate2 (medium model)
if command -v whisper-ctranslate2 >/dev/null 2>&1 || command -v uvx >/dev/null 2>&1; then
    if command -v uvx >/dev/null 2>&1; then
        WHISPER_CMD="uvx whisper-ctranslate2 --model=medium --output_format=txt '$AUDIO_FILE' --output_dir='$RESULTS_DIR'"
        run_benchmark "whisper-ctranslate2 (medium)" "$WHISPER_CMD"
    else
        WHISPER_CMD="whisper-ctranslate2 --model=medium --output_format=txt '$AUDIO_FILE' --output_dir='$RESULTS_DIR'"
        run_benchmark "whisper-ctranslate2 (medium)" "$WHISPER_CMD"
    fi
else
    echo "whisper-ctranslate2 not found, skipping..."
    echo "Install: uv tool install whisper-ctranslate2"
fi

# Benchmark openai-whisper (turbo model)
if command -v whisper >/dev/null 2>&1; then
    WHISPER_CMD="whisper --model=turbo --output_format=txt '$AUDIO_FILE' --output_dir='$RESULTS_DIR'"
    run_benchmark "whisper (turbo)" "$WHISPER_CMD"
else
    echo "openai-whisper not found, skipping..."
    echo "Install: pip install openai-whisper"
fi

# Copy temp results to final
cat "$RESULTS_FILE.tmp" >> "$RESULTS_FILE" 2>/dev/null || true
rm -f "$RESULTS_FILE.tmp"

echo "" >> "$RESULTS_FILE"
echo "## Notes" >> "$RESULTS_FILE"
echo "" >> "$RESULTS_FILE"
echo "- **chough**: Uses Parakeet TDT 0.6b V3 model with 60s chunked processing" >> "$RESULTS_FILE"
echo "- **whisper-ctranslate2**: Uses Faster-Whisper with CTranslate2 backend" >> "$RESULTS_FILE"
echo "- **whisper**: OpenAI's official implementation" >> "$RESULTS_FILE"
echo "- Realtime factor = audio_duration / processing_time (higher is better)" >> "$RESULTS_FILE"
echo "- Memory measured as peak RSS ( Resident Set Size)" >> "$RESULTS_FILE"

echo ""
echo "=========================================="
echo "Results saved to: $RESULTS_FILE"
echo "=========================================="

cat "$RESULTS_FILE"
