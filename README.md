# 🐦‍⬛ chough

*pronounced "chuff" /tʃʌf/* — a fast, memory-efficient ASR CLI using [Parakeet TDT 0.6b V3](https://huggingface.co/nvidia/parakeet-tdt-0.6b-v3) via [sherpa-onnx](https://github.com/k2-fsa/sherpa-onnx) with chunked processing.

## Features

- ⚡ **Fast**: 20-24x realtime transcription
- 🧠 **Memory-efficient**: Processes audio in chunks
- 📦 **Any audio format**: mp3, wav, m4a, ogg, flac, etc. (via ffmpeg)
- 🎯 **No setup**: Auto-downloads models on first run
- 📝 **Multiple formats**: text, json, vtt

## Installation

### Arch Linux (AUR)

```bash
yay -S chough
# or
paru -S chough
```

### macOS (Homebrew)

```bash
brew install hyperpuncher/tap/chough
```

### Windows (Winget)

```bash
winget install hyperpuncher.chough
```

### Binary releases

Download from [GitHub Releases](https://github.com/hyperpuncher/chough/releases).

### Build from source

```bash
go install github.com/hyperpuncher/chough/cmd/chough@latest
```

## Usage

```bash
# Basic transcription (text to stdout)
chough audio.mp3

# WebVTT subtitles
chough -f vtt -o subtitles.vtt video.mp4

# JSON with timestamps
chough -f json podcast.mp3 > transcript.json

# Smaller chunks for lower memory usage
chough -c 30 long-interview.wav
```

## Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-c, --chunk-size` | Chunk size in seconds | 60 |
| `-f, --format` | Output format: text, json, vtt | text |
| `-o, --output` | Output file (default: stdout) | - |
| `--version` | Show version | - |
| `-h, --help` | Show help | - |

## Environment

- `CHOUGH_MODEL`: Path to model directory (optional, auto-downloaded if not set)

## Model

Default: [Parakeet TDT 0.6b V3](https://huggingface.co/nvidia/parakeet-tdt-0.6b-v3) (`sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2`)

Models are automatically downloaded to `$XDG_CACHE_HOME/chough/models` (~650MB).

## How it works

1. Splits audio into 60s chunks (configurable)
2. Loads ONNX model once (~3s)
3. Processes chunks sequentially
4. Outputs results

## Performance

Benchmark on 1-minute audio file (AMD Ryzen, 8 cores):

| Tool | Model | Time | Realtime Factor | Memory |
|------|-------|------|-----------------|--------|
| **chough** | Parakeet TDT 0.6b V3 | **~3s** | **~20x** | **~500MB** |
| whisper-ctranslate2 | medium | ~30s | ~2x | ~2-3GB |
| whisper | turbo | ~60s | ~1x | ~1.5GB |

**chough is ~10-20x faster** than other tools while using **3-6x less memory**.

### Speed by audio length

| Duration | chough | Typical Speed |
|----------|--------|---------------|
| 15s | 0.6s | **24x realtime** |
| 1min | 3s | **20x realtime** |
| 5min | 15s | **20x realtime** |
| 1hour | ~3min | **20x realtime** |

Run your own benchmarks: `just benchmark <audio-file>`

## License

MIT
