package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/hyperpuncher/chough/internal/server"
	"github.com/hyperpuncher/chough/internal/transcribe"
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
	AudioFile   string
	ChunkSize   int
	Format      string
	OutputFile  string
	ShowVersion bool
	RemoteMode  bool

	ServerMode    bool
	ServerOptions server.ServerOptions
}

type cliFlag struct {
	short string
	long  string
	arg   string
}

type usageRow struct {
	label      string
	plainLabel string
	desc       string
}

var usageFlags = []cliFlag{
	{short: "c", long: "chunk-size", arg: "int"},
	{short: "f", long: "format", arg: "string"},
	{short: "o", long: "output", arg: "file"},
	{short: "r", long: "remote"},
	{long: "version"},
}

var serverFlags = []cliFlag{
	{long: "server"},
	{long: "host", arg: "string"},
	{long: "port", arg: "int"},
	{long: "workers", arg: "int"},
	{long: "queue-size", arg: "int"},
	{long: "max-upload", arg: "int"},
	{long: "allow-private-urls"},
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

func newCLIFlagSet() (*flag.FlagSet, *cliOptions) {
	fs := flag.NewFlagSet("chough", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	opts := &cliOptions{}
	defaults := server.DefaultOptions()

	fs.IntVar(&opts.ChunkSize, "c", transcribe.DefaultChunkSize, "chunk size in seconds")
	fs.IntVar(&opts.ChunkSize, "chunk-size", transcribe.DefaultChunkSize, "chunk size in seconds")
	fs.StringVar(&opts.Format, "f", "text", "output format: text, json, vtt")
	fs.StringVar(&opts.Format, "format", "text", "output format: text, json, vtt")
	fs.StringVar(&opts.OutputFile, "o", "", "output file")
	fs.StringVar(&opts.OutputFile, "output", "", "output file")
	fs.BoolVar(&opts.ShowVersion, "version", false, "show version")
	fs.BoolVar(&opts.RemoteMode, "r", false, "transcribe via remote server using CHOUGH_URL")
	fs.BoolVar(&opts.RemoteMode, "remote", false, "transcribe via remote server using CHOUGH_URL")

	fs.BoolVar(&opts.ServerMode, "server", false, "run in server mode")
	fs.StringVar(&opts.ServerOptions.Host, "host", defaults.Host, "server host")
	fs.IntVar(&opts.ServerOptions.Port, "port", defaults.Port, "server port")
	fs.IntVar(&opts.ServerOptions.Workers, "workers", defaults.Workers, "concurrent preparation workers")
	fs.IntVar(&opts.ServerOptions.MaxQueueSize, "queue-size", defaults.MaxQueueSize, "maximum queued requests")
	fs.Int64Var(&opts.ServerOptions.MaxUploadMB, "max-upload", defaults.MaxUploadMB, "max upload size in MB")
	fs.BoolVar(&opts.ServerOptions.AllowPrivateURLs, "allow-private-urls", false, "allow URL downloads from private networks")
	fs.Usage = func() { printUsage(os.Stderr) }
	return fs, opts
}

func parseCLI(args []string) (cliOptions, error) {
	fs, opts := newCLIFlagSet()
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return cliOptions{}, errShowHelp
		}
		return cliOptions{}, fmt.Errorf("%w: %v", errInvalidArgs, err)
	}
	opts.Format = strings.ToLower(opts.Format)

	setFlags := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })
	flagSet := func(names ...string) bool {
		for _, name := range names {
			if setFlags[name] {
				return true
			}
		}
		return false
	}

	if opts.ShowVersion {
		return *opts, nil
	}
	if opts.ServerMode {
		if fs.NArg() != 0 {
			return cliOptions{}, fmt.Errorf("%w: server mode does not accept an audio file", errInvalidArgs)
		}
		if opts.RemoteMode || flagSet("c", "chunk-size", "f", "format", "o", "output") {
			return cliOptions{}, fmt.Errorf("%w: transcription flags cannot be used with --server", errInvalidArgs)
		}
		return *opts, nil
	}
	if flagSet("host", "port", "workers", "queue-size", "max-upload", "allow-private-urls") {
		return cliOptions{}, fmt.Errorf("%w: server flags require --server", errInvalidArgs)
	}
	if err := transcribe.ValidateChunkSize(opts.ChunkSize); err != nil {
		return cliOptions{}, fmt.Errorf("%w: %v", errInvalidArgs, err)
	}

	if fs.NArg() < 1 {
		stat, err := os.Stdin.Stat()
		if err != nil || (stat.Mode()&os.ModeCharDevice) != 0 {
			return cliOptions{}, fmt.Errorf("%w: audio file is required (or pipe audio to stdin)", errInvalidArgs)
		}
		opts.AudioFile = "-"
	} else {
		if fs.NArg() > 1 {
			extra := fs.Arg(1)
			if strings.HasPrefix(extra, "-") {
				return cliOptions{}, fmt.Errorf("%w: unexpected flag %q; flags must appear before the audio file", errInvalidArgs, extra)
			}
			return cliOptions{}, fmt.Errorf("%w: unexpected argument %q; only one audio file is accepted", errInvalidArgs, extra)
		}
		opts.AudioFile = fs.Arg(0)
	}

	switch opts.Format {
	case "text", "json", "vtt":
		return *opts, nil
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

func printAlignedRows(out io.Writer, rows []usageRow) {
	maxLabelWidth := 0
	for _, row := range rows {
		if w := utf8.RuneCountInString(row.plainLabel); w > maxLabelWidth {
			maxLabelWidth = w
		}
	}

	for _, row := range rows {
		pad := strings.Repeat(" ", maxLabelWidth-utf8.RuneCountInString(row.plainLabel))
		_, _ = fmt.Fprintf(out, "  %s%s  %s\n", row.label, pad, row.desc)
	}
}

func printFlagRows(out io.Writer, fs *flag.FlagSet, flags []cliFlag) {
	rows := make([]usageRow, 0, len(flags))
	for _, f := range flags {
		registered := fs.Lookup(f.long)
		description := registered.Usage
		value := registered.DefValue
		if f.long == "output" {
			value = "stdout"
		}
		if f.arg != "" {
			description += fmt.Sprintf(" %s(default: %s)%s", dim, value, reset)
		}
		rows = append(rows, usageRow{
			label:      coloredFlagLabel(f),
			plainLabel: plainFlagLabel(f),
			desc:       description,
		})
	}
	printAlignedRows(out, rows)
}

func printUsage(out io.Writer) {
	fs, _ := newCLIFlagSet()
	_, _ = fmt.Fprintf(out, "%s🐦‍⬛ %schough%s\n\n", bold, magenta, reset)

	_, _ = fmt.Fprintf(out, "%sCLI Usage:%s\n", bold, reset)
	_, _ = fmt.Fprintln(out, "  chough [flags] [audio-file]")
	_, _ = fmt.Fprintln(out, "  cat audio | chough [flags]")
	_, _ = fmt.Fprintln(out)

	_, _ = fmt.Fprintf(out, "%sCLI Flags:%s\n", bold, reset)
	printFlagRows(out, fs, usageFlags)
	_, _ = fmt.Fprintln(out)

	_, _ = fmt.Fprintf(out, "%sServer Usage:%s\n", bold, reset)
	_, _ = fmt.Fprintln(out, "  chough --server [flags]")
	_, _ = fmt.Fprintln(out)

	_, _ = fmt.Fprintf(out, "%sServer Flags:%s\n", bold, reset)
	printFlagRows(out, fs, serverFlags)
	_, _ = fmt.Fprintln(out)

	_, _ = fmt.Fprintf(out, "%sExamples:%s\n", bold, reset)
	exampleRows := []usageRow{
		{label: fmt.Sprintf("%s$%s chough audio.mp3", green, reset), plainLabel: "$ chough audio.mp3", desc: fmt.Sprintf("%s# default chunks, text output%s", dim, reset)},
		{label: fmt.Sprintf("%s$%s cat audio.mp3 | chough", green, reset), plainLabel: "$ cat audio.mp3 | chough", desc: fmt.Sprintf("%s# transcribe from pipe%s", dim, reset)},
		{label: fmt.Sprintf("%s$%s chough -c 30 talk.mp3", green, reset), plainLabel: "$ chough -c 30 talk.mp3", desc: fmt.Sprintf("%s# 30s chunks%s", dim, reset)},
		{label: fmt.Sprintf("%s$%s CHOUGH_URL=http://localhost:8080 chough -r audio.mp3", green, reset), plainLabel: "$ CHOUGH_URL=http://localhost:8080 chough -r audio.mp3", desc: fmt.Sprintf("%s# transcribe via remote server%s", dim, reset)},
		{label: fmt.Sprintf("%s$%s chough -f vtt -o subs.vtt audio.mp3", green, reset), plainLabel: "$ chough -f vtt -o subs.vtt audio.mp3", desc: fmt.Sprintf("%s# WebVTT to file%s", dim, reset)},
		{label: fmt.Sprintf("%s$%s chough --server --port 8080", green, reset), plainLabel: "$ chough --server --port 8080", desc: fmt.Sprintf("%s# Run server on port 8080%s", dim, reset)},
	}
	printAlignedRows(out, exampleRows)
	_, _ = fmt.Fprintln(out)

	_, _ = fmt.Fprintf(out, "%sEnvironment:%s\n", bold, reset)
	envRows := []usageRow{
		{label: fmt.Sprintf("%sCHOUGH_MODEL%s", cyan, reset), plainLabel: "CHOUGH_MODEL", desc: fmt.Sprintf("path to model dir %s(optional, auto-downloaded if not set)%s", dim, reset)},
		{label: fmt.Sprintf("%sCHOUGH_URL%s", cyan, reset), plainLabel: "CHOUGH_URL", desc: fmt.Sprintf("remote server URL %s(required with --remote, must start with http:// or https://)%s", dim, reset)},
	}
	printAlignedRows(out, envRows)
}
