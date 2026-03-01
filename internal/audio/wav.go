package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Wave represents audio data
type Wave struct {
	Samples    []float32
	SampleRate int
}

// ReadWave reads a WAV file and returns the audio data
// This is a pure Go implementation to avoid C memory leaks
func ReadWave(filename string) (*Wave, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read RIFF header
	var riffHeader [12]byte
	if _, err := io.ReadFull(file, riffHeader[:]); err != nil {
		return nil, fmt.Errorf("failed to read RIFF header: %w", err)
	}

	// Verify RIFF header
	if string(riffHeader[0:4]) != "RIFF" {
		return nil, fmt.Errorf("not a valid WAV file (no RIFF header), got: %s", string(riffHeader[0:4]))
	}

	// Verify WAVE format
	if string(riffHeader[8:12]) != "WAVE" {
		return nil, fmt.Errorf("not a valid WAV file (no WAVE format), got: %s", string(riffHeader[8:12]))
	}

	var (
		sampleRate    int
		numChannels   int
		bitsPerSample int
		dataSize      uint32
	)

	// Parse chunks
	for {
		// Read chunk header
		var chunkHeader [8]byte
		_, err := io.ReadFull(file, chunkHeader[:])
		if err == io.EOF {
			return nil, fmt.Errorf("no data chunk found in WAV file")
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read chunk header: %w", err)
		}

		chunkID := string(chunkHeader[0:4])
		chunkSize := binary.LittleEndian.Uint32(chunkHeader[4:8])

		switch chunkID {
		case "fmt ":
			// Read fmt chunk
			fmtData := make([]byte, chunkSize)
			if _, err := io.ReadFull(file, fmtData); err != nil {
				return nil, fmt.Errorf("failed to read fmt chunk: %w", err)
			}

			// Parse fmt chunk
			audioFormat := binary.LittleEndian.Uint16(fmtData[0:2])
			if audioFormat != 1 {
				return nil, fmt.Errorf("unsupported audio format: %d (only PCM supported)", audioFormat)
			}

			numChannels = int(binary.LittleEndian.Uint16(fmtData[2:4]))
			sampleRate = int(binary.LittleEndian.Uint32(fmtData[4:8]))
			// Skip byte rate (8-12) and block align (12-14)
			bitsPerSample = int(binary.LittleEndian.Uint16(fmtData[14:16]))

			if numChannels != 1 {
				return nil, fmt.Errorf("unsupported number of channels: %d (only mono supported)", numChannels)
			}
			if bitsPerSample != 16 {
				return nil, fmt.Errorf("unsupported bits per sample: %d (only 16-bit supported)", bitsPerSample)
			}

		case "data":
			dataSize = chunkSize
			// Read the audio data now
			return readSamples(file, sampleRate, dataSize)

		default:
			// Skip unknown chunks (LIST, INFO, etc.)
			if _, err := file.Seek(int64(chunkSize), 1); err != nil {
				return nil, fmt.Errorf("failed to skip chunk %s: %w", chunkID, err)
			}
		}
	}
}

func readSamples(file *os.File, sampleRate int, dataSize uint32) (*Wave, error) {
	// Read all samples
	data := make([]byte, dataSize)
	if _, err := io.ReadFull(file, data); err != nil {
		return nil, fmt.Errorf("failed to read audio data: %w", err)
	}

	// Convert int16 samples to float32
	numSamples := int(dataSize) / 2
	samples := make([]float32, numSamples)

	for i := 0; i < numSamples; i++ {
		sample := int16(binary.LittleEndian.Uint16(data[i*2 : (i+1)*2]))
		samples[i] = float32(sample) / 32768.0
	}

	return &Wave{
		Samples:    samples,
		SampleRate: sampleRate,
	}, nil
}
