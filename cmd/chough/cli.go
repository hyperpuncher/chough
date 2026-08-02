package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

var (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	gray    = "\033[38;5;250m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
)

var (
	errShowHelp    = errors.New("show help")
	errInvalidArgs = errors.New("invalid arguments")
)

type cliOptions struct {
	// CLI mode
	AudioFile   string
	ChunkSize   int
	Format      string
	OutputFile  string
	ShowVersion bool
	RemoteMode  bool

	// Server mode
	ServerMode       bool
	ServerHost       string
	ServerPort       int
	Workers          int
	QueueSize        int
	MaxUploadMB      int
	AllowPrivateURLs bool
}

type cliFlag struct {
	short       string
	long        string
	arg         string
	description string
	defaultVal  string
}

type usageRow struct {
	label      string
	plainLabel string
	desc       string
}

var usageFlags = []cliFlag{
	{short: "c", long: "chunk-size", arg: "int", description: "chunk size in seconds", defaultVal: "60"},
	{short: "f", long: "format", arg: "string", description: "output format: text, json, vtt", defaultVal: "text"},
	{short: "o", long: "output", arg: "file", description: "output file", defaultVal: "stdout"},
	{short: "r", long: "remote", description: "transcribe via remote server using CHOUGH_URL"},
	{long: "version", description: "show version"},
}

var serverFlags = []cliFlag{
	{long: "server", description: "run in server mode"},
	{long: "host", arg: "string", description: "server host", defaultVal: "0.0.0.0"},
	{long: "port", arg: "int", description: "server port", defaultVal: "8080"},
	{long: "workers", arg: "int", description: "concurrent preparation workers", defaultVal: "2"},
	{long: "queue-size", arg: "int", description: "maximum queued requests", defaultVal: "10"},
	{long: "max-upload", arg: "int", description: "max upload size in MB", defaultVal: "1024"},
	{long: "allow-private-urls", description: "allow URL downloads from private networks"},
}

func init() {
	if !isStderrTTY() || os.Getenv("NO_COLOR") != "" {
		disableColors()
	}
}

func isStderrTTY() bool {
	info, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func disableColors() {
	reset = ""
	bold = ""
	dim = ""
	gray = ""
	green = ""
	yellow = ""
	magenta = ""
	cyan = ""
}

func parseCLI(args []string) (cliOptions, error) {
	fs := flag.NewFlagSet("chough", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	// CLI flags
	chunkSize := fs.Int("c", 60, "chunk size in seconds")
	fs.IntVar(chunkSize, "chunk-size", 60, "chunk size in seconds")
	format := fs.String("f", "text", "output format (text, json, vtt)")
	fs.StringVar(format, "format", "text", "output format (text, json, vtt)")
	outputFile := fs.String("o", "", "output file")
	fs.StringVar(outputFile, "output", "", "output file")
	showVersion := fs.Bool("version", false, "show version")
	remoteMode := fs.Bool("r", false, "transcribe via remote server using CHOUGH_URL")
	fs.BoolVar(remoteMode, "remote", false, "transcribe via remote server using CHOUGH_URL")

	// Server flags
	serverMode := fs.Bool("server", false, "run in server mode")
	serverHost := fs.String("host", "0.0.0.0", "server host")
	serverPort := fs.Int("port", 8080, "server port")
	workers := fs.Int("workers", 2, "concurrent preparation workers")
	queueSize := fs.Int("queue-size", 10, "maximum queued requests")
	maxUploadMB := fs.Int("max-upload", 1024, "max upload size in MB")
	allowPrivateURLs := fs.Bool("allow-private-urls", false, "allow URL downloads from private networks")

	fs.Usage = func() {
		printUsage()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return cliOptions{}, errShowHelp
		}
		return cliOptions{}, fmt.Errorf("%w: %v", errInvalidArgs, err)
	}

	opts := cliOptions{
		ChunkSize:        *chunkSize,
		Format:           strings.ToLower(*format),
		OutputFile:       *outputFile,
		ShowVersion:      *showVersion,
		RemoteMode:       *remoteMode,
		ServerMode:       *serverMode,
		ServerHost:       *serverHost,
		ServerPort:       *serverPort,
		Workers:          *workers,
		QueueSize:        *queueSize,
		MaxUploadMB:      *maxUploadMB,
		AllowPrivateURLs: *allowPrivateURLs,
	}

	if opts.ShowVersion || opts.ServerMode {
		return opts, nil
	}
	if opts.ChunkSize <= 0 {
		return cliOptions{}, fmt.Errorf("%w: chunk size must be positive", errInvalidArgs)
	}

	if fs.NArg() < 1 {
		// Check if stdin has data for pipe mode
		stat, err := os.Stdin.Stat()
		if err != nil || (stat.Mode()&os.ModeCharDevice) != 0 {
			return cliOptions{}, fmt.Errorf("%w: audio file is required (or pipe audio to stdin)", errInvalidArgs)
		}
		opts.AudioFile = "-"
	} else {
		opts.AudioFile = fs.Arg(0)
	}

	switch opts.Format {
	case "text", "json", "vtt":
		return opts, nil
	default:
		return cliOptions{}, fmt.Errorf("%w: unknown format %q (valid: text, json, vtt)", errInvalidArgs, opts.Format)
	}
}

func formatFlagLabel(f cliFlag) string {
	parts := make([]string, 0, 2)
	if f.short != "" {
		parts = append(parts, "-"+f.short)
	}
	if f.long != "" {
		parts = append(parts, "--"+f.long)
	}
	return strings.Join(parts, ", ")
}

func plainFlagLabel(f cliFlag) string {
	label := formatFlagLabel(f)
	if f.arg != "" {
		label += " " + f.arg
	}
	return label
}

func coloredFlagLabel(f cliFlag) string {
	base := formatFlagLabel(f)
	if f.arg == "" {
		return cyan + base + reset
	}
	return cyan + base + " " + yellow + f.arg + reset
}

func printAlignedRows(rows []usageRow) {
	maxLabelWidth := 0
	for _, row := range rows {
		if w := utf8.RuneCountInString(row.plainLabel); w > maxLabelWidth {
			maxLabelWidth = w
		}
	}

	for _, row := range rows {
		pad := strings.Repeat(" ", maxLabelWidth-utf8.RuneCountInString(row.plainLabel))
		fmt.Fprintf(os.Stderr, "  %s%s  %s\n", row.label, pad, row.desc)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "%s🐦‍⬛ %schough%s\n\n", bold, magenta, reset)

	fmt.Fprintf(os.Stderr, "%sCLI Usage:%s\n", bold, reset)
	fmt.Fprintln(os.Stderr, "  chough [flags] [audio-file]")
	fmt.Fprintln(os.Stderr, "  cat audio | chough [flags]")
	fmt.Fprintln(os.Stderr)

	fmt.Fprintf(os.Stderr, "%sCLI Flags:%s\n", bold, reset)
	flagRows := make([]usageRow, 0, len(usageFlags))
	for _, f := range usageFlags {
		desc := f.description
		if f.defaultVal != "" {
			desc += fmt.Sprintf(" %s(default: %s)%s", dim, f.defaultVal, reset)
		}
		flagRows = append(flagRows, usageRow{
			label:      coloredFlagLabel(f),
			plainLabel: plainFlagLabel(f),
			desc:       desc,
		})
	}
	printAlignedRows(flagRows)
	fmt.Fprintln(os.Stderr)

	fmt.Fprintf(os.Stderr, "%sServer Usage:%s\n", bold, reset)
	fmt.Fprintln(os.Stderr, "  chough --server [flags]")
	fmt.Fprintln(os.Stderr)

	fmt.Fprintf(os.Stderr, "%sServer Flags:%s\n", bold, reset)
	serverRows := make([]usageRow, 0, len(serverFlags))
	for _, f := range serverFlags {
		desc := f.description
		if f.defaultVal != "" {
			desc += fmt.Sprintf(" %s(default: %s)%s", dim, f.defaultVal, reset)
		}
		serverRows = append(serverRows, usageRow{
			label:      coloredFlagLabel(f),
			plainLabel: plainFlagLabel(f),
			desc:       desc,
		})
	}
	printAlignedRows(serverRows)
	fmt.Fprintln(os.Stderr)

	fmt.Fprintf(os.Stderr, "%sExamples:%s\n", bold, reset)
	exampleRows := []usageRow{
		{label: fmt.Sprintf("%s$%s chough audio.mp3", green, reset), plainLabel: "$ chough audio.mp3", desc: fmt.Sprintf("%s# 60s chunks, text output%s", dim, reset)},
		{label: fmt.Sprintf("%s$%s cat audio.mp3 | chough", green, reset), plainLabel: "$ cat audio.mp3 | chough", desc: fmt.Sprintf("%s# transcribe from pipe%s", dim, reset)},
		{label: fmt.Sprintf("%s$%s chough -c 30 talk.mp3", green, reset), plainLabel: "$ chough -c 30 talk.mp3", desc: fmt.Sprintf("%s# 30s chunks%s", dim, reset)},
		{label: fmt.Sprintf("%s$%s CHOUGH_URL=http://localhost:8080 chough -r audio.mp3", green, reset), plainLabel: "$ CHOUGH_URL=http://localhost:8080 chough -r audio.mp3", desc: fmt.Sprintf("%s# transcribe via remote server%s", dim, reset)},
		{label: fmt.Sprintf("%s$%s chough -f vtt -o subs.vtt audio.mp3", green, reset), plainLabel: "$ chough -f vtt -o subs.vtt audio.mp3", desc: fmt.Sprintf("%s# WebVTT to file%s", dim, reset)},
		{label: fmt.Sprintf("%s$%s chough --server --port 8080", green, reset), plainLabel: "$ chough --server --port 8080", desc: fmt.Sprintf("%s# Run server on port 8080%s", dim, reset)},
	}
	printAlignedRows(exampleRows)
	fmt.Fprintln(os.Stderr)

	fmt.Fprintf(os.Stderr, "%sEnvironment:%s\n", bold, reset)
	envRows := []usageRow{
		{label: fmt.Sprintf("%sCHOUGH_MODEL%s", cyan, reset), plainLabel: "CHOUGH_MODEL", desc: fmt.Sprintf("path to model dir %s(optional, auto-downloaded if not set)%s", dim, reset)},
		{label: fmt.Sprintf("%sCHOUGH_URL%s", cyan, reset), plainLabel: "CHOUGH_URL", desc: fmt.Sprintf("remote server URL %s(required with --remote, must start with http:// or https://)%s", dim, reset)},
	}
	printAlignedRows(envRows)
}
