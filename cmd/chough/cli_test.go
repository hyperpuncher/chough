package main

import (
	"errors"
	"strings"
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
	if !opts.ServerMode || opts.ServerOptions.Host != "0.0.0.0" || opts.ServerOptions.Port != 8080 || opts.ServerOptions.Workers != 2 || opts.ServerOptions.MaxQueueSize != 10 || opts.ServerOptions.MaxUploadMB != 1024 || opts.ServerOptions.AllowPrivateURLs {
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

func TestParseCLIRejectsUnexpectedArguments(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"audio.wav", "30"}, want: "only one audio file"},
		{args: []string{"audio.wav", "--format", "json"}, want: "flags must appear before"},
	} {
		_, err := parseCLI(test.args)
		if !errors.Is(err, errInvalidArgs) || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("parseCLI(%q) error = %v, want errInvalidArgs containing %q", test.args, err, test.want)
		}
	}
}

func TestParseCLIRejectsInvalidModeCombinations(t *testing.T) {
	for _, args := range [][]string{
		{"--host", "127.0.0.1", "audio.wav"},
		{"--server", "audio.wav"},
		{"--server", "--remote"},
		{"--server", "--chunk-size", "30"},
	} {
		if _, err := parseCLI(args); !errors.Is(err, errInvalidArgs) {
			t.Fatalf("parseCLI(%q) error = %v, want errInvalidArgs", args, err)
		}
	}
}

func TestParseCLIHelp(t *testing.T) {
	_, err := parseCLI([]string{"--help"})
	if !errors.Is(err, errShowHelp) {
		t.Fatalf("parseCLI() error = %v, want errShowHelp", err)
	}
}
