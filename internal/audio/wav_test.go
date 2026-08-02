package audio

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestReadWave(t *testing.T) {
	path := writeWaveFixture(t, []int16{-32768, 0, 16384, 32767})

	wave, err := ReadWave(path)
	if err != nil {
		t.Fatalf("ReadWave() error = %v", err)
	}
	if wave.SampleRate != 16000 {
		t.Fatalf("SampleRate = %d, want 16000", wave.SampleRate)
	}
	if len(wave.Samples) != 4 {
		t.Fatalf("len(Samples) = %d, want 4", len(wave.Samples))
	}
	if math.Abs(float64(wave.Samples[2]-0.5)) > 0.0001 {
		t.Fatalf("Samples[2] = %f, want 0.5", wave.Samples[2])
	}
}

func TestReadWaveRejectsMalformedInput(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "truncated header", data: []byte("RIFF")},
		{name: "wrong container", data: []byte("NOPE\x00\x00\x00\x00WAVE")},
		{name: "short fmt chunk", data: waveWithChunks(chunk("fmt ", make([]byte, 8)))},
		{name: "data before fmt", data: waveWithChunks(chunk("data", []byte{0, 0}))},
		{name: "odd 16-bit data", data: waveWithChunks(pcmFormatChunk(), chunk("data", []byte{0}))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input.wav")
			if err := os.WriteFile(path, tt.data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadWave(path); err == nil {
				t.Fatal("ReadWave() error = nil, want malformed input error")
			}
		})
	}
}

func writeWaveFixture(t *testing.T, samples []int16) string {
	t.Helper()
	var data bytes.Buffer
	if err := binary.Write(&data, binary.LittleEndian, samples); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "input.wav")
	if err := os.WriteFile(path, waveWithChunks(pcmFormatChunk(), chunk("data", data.Bytes())), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func pcmFormatChunk() []byte {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint16(data[0:2], 1)
	binary.LittleEndian.PutUint16(data[2:4], 1)
	binary.LittleEndian.PutUint32(data[4:8], 16000)
	binary.LittleEndian.PutUint32(data[8:12], 32000)
	binary.LittleEndian.PutUint16(data[12:14], 2)
	binary.LittleEndian.PutUint16(data[14:16], 16)
	return chunk("fmt ", data)
}

func chunk(id string, data []byte) []byte {
	var out bytes.Buffer
	out.WriteString(id)
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(data)))
	out.Write(data)
	return out.Bytes()
}

func waveWithChunks(chunks ...[]byte) []byte {
	var body bytes.Buffer
	body.WriteString("WAVE")
	for _, chunk := range chunks {
		body.Write(chunk)
	}

	var out bytes.Buffer
	out.WriteString("RIFF")
	_ = binary.Write(&out, binary.LittleEndian, uint32(body.Len()))
	out.Write(body.Bytes())
	return out.Bytes()
}
