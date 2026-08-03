# 🐦‍⬛ chough

_pronounced "chuff" /tʃʌf/_ — a fast, memory-efficient ASR CLI using [Parakeet TDT 0.6b V3](https://huggingface.co/nvidia/parakeet-tdt-0.6b-v3) via [sherpa-onnx](https://github.com/k2-fsa/sherpa-onnx) with chunked processing.

## Features

- ⚡ **Fast**: 6-21x realtime transcription, including startup
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

# Pipe audio from stdin
cat audio.mp3 | chough

# Video files work too - extracts audio automatically
chough -f vtt -o subtitles.vtt lecture.mp4

# JSON with timestamps
chough -f json podcast.mp3 > transcript.json

# Smaller chunks for lower memory usage
chough -c 30 long-interview.wav

# Use remote server mode (requires CHOUGH_URL)
CHOUGH_URL=http://localhost:8080 chough --remote audio.mp3
```

### Flags

| Flag               | Description                      | Default |
| ------------------ | -------------------------------- | ------- |
| `-c, --chunk-size` | Chunk size in seconds            | 60      |
| `-f, --format`     | Output format: text, json, vtt   | text    |
| `-o, --output`     | Output file                      | stdout  |
| `-r, --remote`     | Transcribe via CHOUGH_URL server | -       |
| `--version`        | Show version                     | -       |
| `-h, --help`       | Show help                        | -       |

## Server Mode

Run `chough` as an HTTP server for API access. The server keeps the model loaded in memory (~1.6GB), eliminating the ~1.5s startup time per request.

Server mode listens on all network interfaces by default for LAN use. It does not provide authentication, TLS, CORS policy, or rate limiting. Do not expose it directly to the public internet; use a trusted reverse proxy when those controls are required.

```bash
# Start server
chough --server --port 8080

# With custom settings
chough --server --host 0.0.0.0 --port 8080 --workers 2 --queue-size 10
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

# Transcribe from a public URL
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

| Flag                   | Description                                | Default |
| ---------------------- | ------------------------------------------ | ------- |
| `--server`             | Run in server mode                         | -       |
| `--host`               | Server host                                | 0.0.0.0 |
| `--port`               | Server port                                | 8080    |
| `--workers`            | Concurrent preparation workers             | 2       |
| `--queue-size`         | Maximum waiting requests                   | 10      |
| `--max-upload`         | Maximum audio size in MB                   | 1024    |
| `--allow-private-urls` | Permit URL downloads from private networks | false   |

URL transcription allows public HTTP and HTTPS destinations. Loopback, private, link-local, multicast, unspecified, and other special-use destinations are blocked, including after redirects. `--allow-private-urls` disables that restriction for trusted LAN or homelab sources.

The default 1024MB limit applies to uploaded, downloaded, or decoded audio. HTTP request bodies may be larger only for multipart framing or base64 encoding overhead. Requests are admitted before their bodies are read, and each transcription has a ten-minute deadline. Workers may prepare audio concurrently, but native recognition is serialized because sherpa-onnx does not document concurrent recognizer safety.

A failure in any chunk fails the whole transcription; the API never returns a partial transcript as a successful result.

### JSON token data

JSON chunks always include `start_time`, `end_time`, and `text`. They may also include aligned token metadata:

| Field        | Meaning                      |
| ------------ | ---------------------------- |
| `tokens`     | Recognizer tokens            |
| `timestamps` | Token start times in seconds |
| `durations`  | Token durations in seconds   |
| `log_probs`  | Token log probabilities      |

Optional token arrays are omitted when sherpa does not provide data aligned with the tokens. Existing response fields remain unchanged.

### Docker

```bash
# Run the published image
docker run -d -p 8080:8080 \
  -v chough-models:/home/chough/.cache/chough/models \
  ghcr.io/hyperpuncher/chough:latest

# Or use Docker Compose
docker compose up -d
```

The release container runs as the non-root `chough` user with fixed UID/GID `10001`. Its persistent model cache is `/home/chough/.cache/chough/models`; mount that directory when using a bind mount or named volume.

## Environment

- `CHOUGH_MODEL`: Path to model directory (optional, auto-downloaded if not set)
- `CHOUGH_URL`: Remote server URL for `--remote` mode (must start with `http://` or `https://`)

## Model

Default: [Parakeet TDT 0.6b V3](https://huggingface.co/nvidia/parakeet-tdt-0.6b-v3) (`sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2`)

Models are automatically downloaded under the operating system's user cache directory at `chough/models/<model-name>` (~650MB). On Linux this is normally `~/.cache/chough/models/<model-name>`.

## How it works

1. Loads the ONNX model once per process.
2. Probes the source and divides it into configurable chunks.
3. Uses ffmpeg to normalize each chunk to 16kHz mono WAV.
4. Transcribes every chunk, failing the operation if any chunk fails.
5. Combines the chunks and writes text, JSON, or WebVTT.

## Performance

### Tool comparison

Benchmark on a 1-minute audio file (AMD Ryzen 5 5600X, 6 cores). The chough row uses the current cold benchmark; the comparison rows retain their original measurements.

| Tool                | Model                |     Time |  Relative | Realtime Factor |    Memory |
| ------------------- | -------------------- | -------: | --------: | --------------: | --------: |
| **chough**          | Parakeet TDT 0.6b V3 | **4.6s** | **12.2x** |       **13.0x** | **1.4GB** |
| whisper-ctranslate2 | medium               |    27.8s |      2.0x |            2.2x |     1.7GB |
| whisper             | turbo                |    56.6s |      1.0x |            1.1x |     5.3GB |

**chough is ~6-12x faster** than the compared tools.

### Speed by audio length

Current cold CLI results using one warmup and three measured runs:

| Duration |  Time |              Speed | Peak RSS |
| -------- | ----: | -----------------: | -------: |
| 15s      |  2.3s |  **6.6x realtime** |   1101MB |
| 1m       |  4.6s | **13.0x realtime** |   1448MB |
| 5m       | 16.4s | **18.3x realtime** |   1894MB |
| 30m      | 94.2s | **19.6x realtime** |   1931MB |

### Current cold and warm measurements

Measured on the same AMD Ryzen 5 5600X with four sherpa threads. Cold runs include process and model startup; warm runs use an already-ready HTTP server. Each result uses one warmup followed by three measured runs.

| Mode        | Audio | Mean | Realtime | Peak RSS |
| ----------- | ----- | ---: | -------: | -------: |
| cold CLI    | 15s   | 2.3s |     6.6x |   1101MB |
| cold CLI    | 1m    | 4.6s |    13.0x |   1448MB |
| warm server | 15s   | 0.7s |    21.3x |   1223MB |
| warm server | 1m    | 3.1s |    19.5x |   1877MB |

Peak warm-server RSS is the server high-water mark after repeated requests. Results vary with CPU, model cache state, and background load.

Run the maintained 15-second and 1-minute benchmark suite:

```bash
just build
just benchmark
```

Compare two binaries or change the measured run count:

```bash
just benchmark --baseline /tmp/chough-before --candidate ./dist/chough
just benchmark --runs 5
```

The runner writes Markdown, JSON, raw hyperfine data, server logs, binary hashes, CPU metadata, and thread metadata under `/tmp/chough-benchmarks` by default. See `benchmarks/benchmark.sh --help` for cold-only, warm-only, fixture, and output options.

## License

MIT
