# Benchmarks

Benchmark scripts to compare chough against other ASR tools.

## Prerequisites

```bash
# Install hyperfine (required for detailed benchmarks)
cargo install hyperfine
# or: pacman -S hyperfine / brew install hyperfine / apt install hyperfine

# Install whisper tools for comparison
uv tool install whisper-ctranslate2      # For faster-whisper
pip install openai-whisper              # For openai-whisper
```

## Usage

### Using just (recommended)

```bash
# Full benchmark with hyperfine (3 runs, statistical analysis)
just benchmark test/audio/audio-5min.wav

# Quick benchmark (single run)
just benchmark-quick test/audio/audio-5min.wav

# Memory profiling only
just profile-mem test/audio/audio-5min.wav
```

### Direct script usage

```bash
# Full hyperfine benchmark
./benchmarks/benchmark-hyperfine.sh test/audio/audio-5min.wav

# Simple benchmark
./benchmarks/benchmark.sh test/audio/audio-5min.wav
```

## What gets measured

| Metric | Tool | Description |
|--------|------|-------------|
| **Time** | hyperfine | Mean, min, max, std dev across runs |
| **Realtime factor** | calculated | audio_duration / processing_time |
| **Memory** | /usr/bin/time | Peak RSS (resident set size) |
| **CPU** | /usr/bin/time | CPU percentage |

## Tools compared

- **chough** - Parakeet TDT 0.6b V3 with chunked processing
- **whisper-ctranslate2** (medium) - Faster-Whisper with CTranslate2 backend
- **whisper** (turbo) - OpenAI's official implementation

## Example output

```markdown
| Tool | Time | Realtime | Memory |
|------|------|----------|--------|
| chough | 14.5s | 20.7x | 520MB |
| whisper-ctranslate2 | 45s | 6.7x | 2.1GB |
| whisper | 120s | 2.5x | 1.8GB |
```

Results are saved to `benchmarks/results/` with timestamps.
