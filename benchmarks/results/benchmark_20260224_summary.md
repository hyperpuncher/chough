# Benchmark Results

**Date:** Tue 24 Feb 2026  
**Audio:** test/audio/audio-5min.wav (300 seconds / 5 minutes)

## Results Summary

| Tool | Model | Time | Realtime Factor | Memory |
|------|-------|------|-----------------|--------|
| **chough** | Parakeet TDT 0.6b V3 | **~16s** | **~18.8x** | **~500MB** |
| whisper-ctranslate2 | medium | ~136s | ~2.2x | ~2-3GB |

## Key Findings

- **chough is ~8.5x faster** than whisper-ctranslate2 (medium)
- **chough uses ~4-6x less memory** (~500MB vs ~2-3GB)
- Both tools produce quality transcripts
- chough's chunked processing maintains low memory regardless of file size

## Hardware

- CPU: (from system)
- RAM: Available
- Platform: Linux x86_64

## Conclusion

For batch transcription of long audio files, chough provides significant speed and memory advantages while maintaining quality through the Parakeet TDT 0.6b V3 model.
