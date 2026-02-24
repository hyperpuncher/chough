# 🐦‍⬛ chough

Fast, memory-efficient ASR CLI using Parakeet TDT 0.6b V3 via sherpa-onnx with chunked processing.

## Features

- ⚡ **Fast**: 20-24x realtime transcription
- 🧠 **Memory-efficient**: Processes audio in chunks
- 📦 **Any audio format**: mp3, wav, m4a, ogg, flac, etc. (via ffmpeg)
- 🎯 **No setup**: Auto-downloads models on first run
- 📝 **Multiple formats**: text, json, vtt

## Installation

### Homebrew

```bash
brew tap hyperpuncher/chough
brew install chough
```

### Binary releases

Download from [GitHub Releases](https://github.com/hyperpuncher/chough/releases):

```bash
# Linux AMD64
curl -L https://github.com/hyperpuncher/chough/releases/latest/download/chough-linux-amd64 -o chough
chmod +x chough
sudo mv chough /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/hyperpuncher/chough/releases/latest/download/chough-darwin-amd64 -o chough
chmod +x chough
sudo mv chough /usr/local/bin/

# macOS (Apple Silicon)
curl -L https://github.com/hyperpuncher/chough/releases/latest/download/chough-darwin-arm64 -o chough
chmod +x chough
sudo mv chough /usr/local/bin/
```

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

Default model: `sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2`

Models are automatically downloaded to `$XDG_CACHE_HOME/chough/models` (~650MB).

## How it works

1. Splits audio into 60s chunks (configurable)
2. Loads ONNX model once (~3s)
3. Processes chunks sequentially
4. Outputs results

## Performance

| File | Duration | Processing Time | Speed |
|------|----------|-----------------|-------|
| Short | 15s | 0.6s | 24x |
| Medium | 5min | 15s | 20x |
| Long | 1hour | ~3min | 20x |

## License

MIT
