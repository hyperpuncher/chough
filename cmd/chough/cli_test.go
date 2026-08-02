package main

import (
	"errors"
	"testing"
)

func TestParseCLI(t *testing.T) {
	opts, err := parseCLI([]string{"--chunk-size", "30", "--format", "VTT", "--output", "out.vtt", "audio.wav"})
	if err != nil {
		t.Fatalf("parseCLI() error = %v", err)
	}
	if opts.AudioFile != "audio.wav" || opts.ChunkSize != 30 || opts.Format != "vtt" || opts.OutputFile != "out.vtt" {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestParseCLIServerDefaults(t *testing.T) {
	opts, err := parseCLI([]string{"--server"})
	if err != nil {
		t.Fatalf("parseCLI() error = %v", err)
	}
	if !opts.ServerMode || opts.ServerHost != "0.0.0.0" || opts.ServerPort != 8080 || opts.Workers != 2 || opts.MaxUploadMB != 1024 {
		t.Fatalf("unexpected server options: %+v", opts)
	}
}

func TestParseCLIRejectsInvalidChunkSize(t *testing.T) {
	_, err := parseCLI([]string{"--chunk-size", "0", "audio.wav"})
	if !errors.Is(err, errInvalidArgs) {
		t.Fatalf("parseCLI() error = %v, want errInvalidArgs", err)
	}
}

func TestParseCLIRejectsInvalidFormat(t *testing.T) {
	_, err := parseCLI([]string{"--format", "xml", "audio.wav"})
	if !errors.Is(err, errInvalidArgs) {
		t.Fatalf("parseCLI() error = %v, want errInvalidArgs", err)
	}
}

func TestParseCLIHelp(t *testing.T) {
	_, err := parseCLI([]string{"--help"})
	if !errors.Is(err, errShowHelp) {
		t.Fatalf("parseCLI() error = %v, want errShowHelp", err)
	}
}
