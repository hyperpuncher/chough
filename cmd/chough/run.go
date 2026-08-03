package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/hyperpuncher/chough/internal/asr"
	"github.com/hyperpuncher/chough/internal/models"
	"github.com/hyperpuncher/chough/internal/output"
	"github.com/hyperpuncher/chough/internal/transcribe"
	"github.com/hyperpuncher/chough/internal/types"
)

func run(ctx context.Context, args []string) error {
	opts, err := parseCLI(args)
	if err != nil {
		switch {
		case errors.Is(err, errShowHelp):
			return nil
		case errors.Is(err, errInvalidArgs):
			printUsage()
			return err
		default:
			return err
		}
	}

	if opts.ShowVersion {
		fmt.Println(version)
		return nil
	}

	// Server mode
	if opts.ServerMode {
		return runServer(ctx, &opts)
	}

	// Handle stdin input by copying to temp file
	audioFile := opts.AudioFile
	var tempFile string
	if opts.AudioFile == "-" {
		tempFile, err = copyStdinToTemp()
		if err != nil {
			return fmt.Errorf("failed to read stdin: %w", err)
		}
		audioFile = tempFile
		defer os.Remove(tempFile)
	}

	var (
		results  []types.ChunkResult
		duration float64
	)

	if opts.RemoteMode {
		serverURL, err := resolveRemoteURL()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "mode: %sremote%s %s•%s url: %s\n", cyan, reset, dim, reset, serverURL)
		srcInfo := opts.AudioFile
		if srcInfo == "-" {
			srcInfo = "stdin"
		}
		fmt.Fprintf(os.Stderr, "audio: %s %s•%s chunks: %ds %s•%s format: %s\n", srcInfo, dim, reset, opts.ChunkSize, dim, reset, opts.Format)

		results, duration, err = transcribeRemote(ctx, serverURL, audioFile, opts.ChunkSize)
		if err != nil {
			return err
		}
	} else {
		fmt.Fprintf(os.Stderr, "mode: %slocal%s\n", cyan, reset)

		recognizer, err := loadRecognizer(ctx)
		if err != nil {
			return err
		}
		defer recognizer.Close()

		service := transcribe.NewService(recognizer)
		transcription, err := transcribeLocal(ctx, service, audioFile, opts.ChunkSize, opts.Format)
		if err != nil {
			return err
		}
		results = transcription.Chunks
		duration = transcription.Duration
		elapsed := transcription.ProcessingTime

		rtFactor := 0.0
		if elapsed > 0 && duration > 0 {
			rtFactor = duration / elapsed.Seconds()
		}
		rtColor := green
		if rtFactor < 10 {
			rtColor = yellow
		}
		fmt.Fprintf(os.Stderr, "%s⚡%s Processed in %s%.1fs%s %s(%s%.1fx%s realtime)%s\n\n",
			yellow, reset, bold, elapsed.Seconds(), reset, dim, rtColor, rtFactor, reset, reset)
	}

	out, closeFn, err := openOutput(opts.OutputFile)
	if err != nil {
		return err
	}

	return writeAndCloseOutput(out, closeFn, opts.Format, results, duration)
}

func loadRecognizer(ctx context.Context) (*asr.Recognizer, error) {
	hideCursor()
	defer showCursor()

	fmt.Fprint(os.Stderr, "⏳ Loading model...\r")
	modelPath, err := models.GetModelPathContext(ctx, modelProgress())
	if err != nil {
		fmt.Fprintln(os.Stderr)
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	recognizer, err := asr.NewRecognizer(asr.DefaultConfig(modelPath))
	if err != nil {
		fmt.Fprintln(os.Stderr)
		return nil, fmt.Errorf("failed to load model: %w", err)
	}

	fmt.Fprintln(os.Stderr, "✅ Model loaded!   ")
	return recognizer, nil
}

func modelProgress() models.ProgressFunc {
	lastBucket := -1
	return func(progress models.Progress) {
		switch progress.Stage {
		case "download":
			if progress.Total <= 0 {
				return
			}
			percent := int(progress.Downloaded * 100 / progress.Total)
			bucket := min(percent/5, 20)
			if bucket == lastBucket {
				return
			}
			lastBucket = bucket
			message := fmt.Sprintf("Downloading model: %.1f / %.1f MB (%d%%)",
				float64(progress.Downloaded)/(1024*1024), float64(progress.Total)/(1024*1024), percent)
			if isStderrTTY() {
				fmt.Fprintf(os.Stderr, "\r%s", message)
			} else {
				fmt.Fprintln(os.Stderr, message)
			}
		case "extract":
			if isStderrTTY() && lastBucket >= 0 {
				fmt.Fprintln(os.Stderr)
			}
			fmt.Fprintln(os.Stderr, "Extracting model...")
		}
	}
}

func transcribeLocal(ctx context.Context, service *transcribe.Service, audioFile string, chunkSize int, format string) (transcribe.Result, error) {
	hideCursor()
	defer showCursor()

	result, err := service.Transcribe(ctx, audioFile, transcribe.Options{
		ChunkSize: chunkSize,
		Progress: func(progress transcribe.Progress) {
			if progress.Completed == 0 {
				fmt.Fprintf(os.Stderr, "audio: %.1fs %s•%s chunks: %ds %s•%s format: %s\n",
					progress.Duration, dim, reset, chunkSize, dim, reset, format)
				return
			}
			percent := float64(progress.Completed) / float64(progress.Total)
			eta := time.Duration(float64(progress.Elapsed)/percent - float64(progress.Elapsed))
			fmt.Fprint(os.Stderr, renderProgressLine(progress.Completed, progress.Total, eta))
		},
	})
	if err != nil {
		return transcribe.Result{}, err
	}
	if len(result.Chunks) > 0 {
		fmt.Fprintln(os.Stderr, renderProgressLine(len(result.Chunks), len(result.Chunks), 0))
	}
	return result, nil
}

func writeAndCloseOutput(out io.Writer, closeFn func() error, format string, results []types.ChunkResult, duration float64) error {
	writeErr := output.Write(out, format, results, duration)
	closeErr := closeFn()
	if writeErr != nil {
		writeErr = fmt.Errorf("error writing output: %w", writeErr)
	}
	if closeErr != nil {
		closeErr = fmt.Errorf("error closing output: %w", closeErr)
	}
	return errors.Join(writeErr, closeErr)
}

func openOutput(path string) (io.Writer, func() error, error) {
	if path == "" {
		return os.Stdout, func() error { return nil }, nil
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating output file: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Output: %s\n", path)
	return file, file.Close, nil
}

// copyStdinToTemp reads all data from stdin and writes it to a temporary file.
// Returns the path to the temp file which the caller must clean up.
func copyStdinToTemp() (string, error) {
	tmpFile, err := os.CreateTemp("", "chough-stdin-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	if _, err := io.Copy(tmpFile, os.Stdin); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to copy stdin: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to close stdin temp file: %w", err)
	}

	return tmpPath, nil
}
