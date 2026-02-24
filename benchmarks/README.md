# Benchmarks

Simple hyperfine-based benchmark.

## Requirements

```bash
cargo install hyperfine
```

## Usage

```bash
just benchmark <audio-file>
```

## Output

Results saved to `benchmarks/results/benchmark_<timestamp>.md`:

```markdown
## Timing

| Tool | Mean | Min | Max | Relative |
|------|------|-----|-----|----------|
| chough | 3.1s | 2.9s | 3.3s | 1.00 |
| whisper-ctranslate2 | 28.5s | 27.1s | 29.8s | 9.19 |

## Memory

| Tool | Memory |
|------|--------|
| chough | 520MB |
| whisper-ctranslate2 | 2450MB |
```
