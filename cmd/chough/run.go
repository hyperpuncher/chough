package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/hyperpuncher/chough/internal/asr"
	"github.com/hyperpuncher/chough/internal/models"
)

func run(args []string) error {
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

	recognizer, err := loadRecognizer()
	if err != nil {
		return err
	}
	defer recognizer.Close()

	duration, err := probeDuration(opts.AudioFile)
	if err != nil {
		return fmt.Errorf("failed to get duration: %w", err)
	}

	boundaries := buildBoundaries(duration, opts.ChunkSize)
	fmt.Fprintf(os.Stderr, "audio: %.1fs %s•%s chunks: %ds %s•%s format: %s\n",
		duration, dim, reset, opts.ChunkSize, dim, reset, opts.Format)

	results, elapsed := transcribeAudio(recognizer, opts.AudioFile, boundaries)

	rtFactor := duration / elapsed.Seconds()
	rtColor := green
	if rtFactor < 10 {
		rtColor = yellow
	}
	fmt.Fprintf(os.Stderr, "%s⚡%s Processed in %s%.1fs%s %s(%s%.1fx%s realtime)%s\n\n",
		yellow, reset, bold, elapsed.Seconds(), reset, dim, rtColor, rtFactor, reset, reset)

	out, closeFn, err := openOutput(opts.OutputFile)
	if err != nil {
		return err
	}
	defer closeFn()

	if err := writeOutput(out, opts.Format, results, duration); err != nil {
		return fmt.Errorf("error writing output: %w", err)
	}
	return nil
}

func loadRecognizer() (*asr.Recognizer, error) {
	hideCursor()
	defer showCursor()

	fmt.Fprint(os.Stderr, "⏳ Loading model...\r")
	modelPath, err := models.GetModelPath()
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

func buildBoundaries(duration float64, chunkSecs int) []float64 {
	chunkCount := int(duration/float64(chunkSecs)) + 1
	boundaries := make([]float64, 0, chunkCount+1)
	for i := 0; i < chunkCount; i++ {
		start := float64(i * chunkSecs)
		if start >= duration {
			break
		}
		boundaries = append(boundaries, start)
	}
	return append(boundaries, duration)
}

func transcribeAudio(recognizer *asr.Recognizer, audioFile string, boundaries []float64) ([]ChunkResult, time.Duration) {
	startTime := time.Now()
	results := make([]ChunkResult, 0, len(boundaries)-1)
	total := len(boundaries) - 1

	hideCursor()
	defer showCursor()

	for i := 0; i < total; i++ {
		chunkStart := boundaries[i]
		chunkEnd := boundaries[i+1]
		if chunkEnd-chunkStart < 0.5 {
			continue
		}

		elapsed := time.Since(startTime)
		percent := float64(i+1) / float64(total)
		eta := time.Duration(float64(elapsed)/percent - float64(elapsed))
		fmt.Fprint(os.Stderr, renderProgressLine(i+1, total, eta))

		result, err := transcribeChunk(recognizer, audioFile, chunkStart, chunkEnd-chunkStart)
		if err != nil {
			fmt.Fprintln(os.Stderr, renderProgressErrorLine(i+1, total, time.Since(startTime), err))
			continue
		}

		results = append(results, ChunkResult{
			StartTime:  chunkStart,
			EndTime:    chunkEnd,
			Text:       result.Text,
			Timestamps: result.Timestamps,
			Tokens:     result.Tokens,
		})
	}

	fmt.Fprintln(os.Stderr, renderProgressLine(total, total, 0))
	return results, time.Since(startTime)
}

func openOutput(path string) (io.Writer, func(), error) {
	if path == "" {
		return os.Stdout, func() {}, nil
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating output file: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Output: %s\n", path)
	return file, func() { file.Close() }, nil
}
