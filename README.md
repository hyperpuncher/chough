# 🐦‍⬛ chough

_pronounced "chuff" /tʃʌf/_ — a fast, memory-efficient ASR CLI using [Parakeet TDT 0.6b V3](https://huggingface.co/nvidia/parakeet-tdt-0.6b-v3) via [sherpa-onnx](https://github.com/k2-fsa/sherpa-onnx) with chunked processing.

## Features

- ⚡ **Fast**: 7-20x realtime transcription
- 🧠 **Memory-efficient**: Processes audio in chunks
- 📦 **Any format**: If `ffmpeg` supports it, `chough` supports it
- 🎯 **No setup**: Auto-downloads models on first run
- 📝 **Multiple formats**: text, json, vtt
- 💻 **CPU only**: No GPU required
- 🌐 **Server mode**: HTTP API for batch processing

## Supported Languages

Bulgarian, Croatian, Czech, Danish, Dutch, English, Estonian, Finnish, French, German, Greek, Hungarian, Italian, Latvian, Lithuanian, Maltese, Polish, Portuguese, Romanian, Slovak, Slovenian, Spanish, Swedish, Russian, Ukrainian

## Requirements

- `ffmpeg` - for audio/video support

## Installation

### Arch Linux (AUR)

```bash
paru -S chough-bin
```

### macOS (Homebrew)

```bash
brew install --cask hyperpuncher/tap/chough
```

### Windows (Winget)

```bash
winget install chough
```

### Binary releases

Download from [GitHub Releases](https://github.com/hyperpuncher/chough/releases).

### Build from source

```bash
go install github.com/hyperpuncher/chough/cmd/chough@latest
```

### Skill

```bash
npx skills add hyperpuncher/dotagents --skill chough
```

## CLI Usage

```bash
# Basic transcription (text to stdout)
chough audio.mp3

# Video files work too - extracts audio automatically
chough -f vtt -o subtitles.vtt lecture.mp4

# JSON with timestamps
chough -f json podcast.mp3 > transcript.json

# Smaller chunks for lower memory usage
chough -c 30 long-interview.wav
```

### Flags

| Flag               | Description                    | Default |
| ------------------ | ------------------------------ | ------- |
| `-c, --chunk-size` | Chunk size in seconds          | 60      |
| `-f, --format`     | Output format: text, json, vtt | text    |
| `-o, --output`     | Output file                    | stdout  |
| `--version`        | Show version                   | -       |
| `-h, --help`       | Show help                      | -       |

## Server Mode

Run `chough` as an HTTP server for API access. The server keeps the model loaded in memory (~1.6GB), eliminating the ~1.5s startup time per request.

```bash
# Start server
chough --server --port 8080

# With custom settings
chough --server --host 0.0.0.0 --port 8080 --workers 2
```

### API Endpoints

| Method | Endpoint      | Description                                    |
| ------ | ------------- | ---------------------------------------------- |
| POST   | `/transcribe` | Transcribe audio (file upload, URL, or base64) |
| GET    | `/health`     | Health check with queue status                 |

### API Examples

```bash
# Upload file
curl -X POST http://localhost:8080/transcribe \
  -F "file=@audio.mp3" \
  -F "format=json" \
  -F "chunk_size=60"

# Transcribe from URL
curl -X POST http://localhost:8080/transcribe \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com/audio.mp3", "format": "vtt"}'

# Base64 audio
curl -X POST http://localhost:8080/transcribe \
  -H "Content-Type: application/json" \
  -d '{"base64": "...", "format": "text"}'

# Health check
curl http://localhost:8080/health
```

### Server Flags

| Flag           | Description          | Default |
| -------------- | -------------------- | ------- |
| `--server`     | Run in server mode   | -       |
| `--host`       | Server host          | 0.0.0.0 |
| `--port`       | Server port          | 8080    |
| `--workers`    | Concurrent workers   | 2       |
| `--max-upload` | Max upload size (MB) | 1024    |

### Docker

```bash
# Build and run
docker build -t chough .
docker run -d -p 8080:8080 chough

# Or use docker-compose
docker-compose up -d
```

## Environment

- `CHOUGH_MODEL`: Path to model directory (optional, auto-downloaded if not set)

## Model

Default: [Parakeet TDT 0.6b V3](https://huggingface.co/nvidia/parakeet-tdt-0.6b-v3) (`sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2`)

Models are automatically downloaded to `$XDG_CACHE_HOME/chough/models` (~650MB).

## How it works

1. Splits audio into 60s chunks (configurable)
2. Loads ONNX model once (~1.5s)
3. Processes chunks sequentially
4. Outputs results

## Performance

Benchmark on 1-minute audio file (AMD Ryzen 5 5600X, 6 cores):

| Tool                | Model                | Time     | Relative  | Realtime Factor | Memory    |
| ------------------- | -------------------- | -------- | --------- | --------------- | --------- |
| **chough**          | Parakeet TDT 0.6b V3 | **4.3s** | **13.2x** | **14.1x**       | **1.6GB** |
| whisper-ctranslate2 | medium               | 27.8s    | 2.0x      | 2.2x            | 1.7GB     |
| whisper             | turbo                | 56.6s    | 1.0x      | 1.1x            | 5.3GB     |

**chough is ~6-13x faster** than other tools.

### Speed by audio length

| Duration | Time  | Speed              |
| -------- | ----- | ------------------ |
| 15s      | 2.0s  | **7.4x realtime**  |
| 1min     | 4.3s  | **14.1x realtime** |
| 5min     | 16.2s | **18.5x realtime** |
| 30min    | 90.2s | **19.9x realtime** |

Run your own benchmarks: `just benchmark <audio-file>`

## License

MIT
