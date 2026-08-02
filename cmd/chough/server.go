package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hyperpuncher/chough/internal/server"
	"github.com/hyperpuncher/chough/internal/transcribe"
	"github.com/hyperpuncher/chough/internal/worker"
)

func runServer(ctx context.Context, opts *cliOptions) error {
	serverOpts := &server.ServerOptions{
		Host:             opts.ServerHost,
		Port:             opts.ServerPort,
		MaxUploadMB:      int64(opts.MaxUploadMB),
		Workers:          opts.Workers,
		MaxQueueSize:     opts.QueueSize,
		AllowPrivateURLs: opts.AllowPrivateURLs,
	}
	if err := server.ValidateOptions(serverOpts); err != nil {
		return fmt.Errorf("invalid server configuration: %w", err)
	}

	// Load model
	hideCursor()
	defer showCursor()

	fmt.Fprint(os.Stderr, "⏳ Loading model...\r")
	recognizer, err := server.LoadRecognizer()
	if err != nil {
		fmt.Fprintln(os.Stderr)
		return err
	}
	defer recognizer.Close()
	fmt.Fprintln(os.Stderr, "✅ Model loaded!   ")

	// Create worker pool
	service := transcribe.NewService(recognizer)
	pool := worker.NewPool(opts.Workers, opts.QueueSize, service)
	defer pool.Shutdown()

	// Create HTTP server
	srv := server.NewServer(serverOpts, pool, version)

	// Start server
	fmt.Fprintf(os.Stderr, "🚀 Server running on http://%s:%d\n", opts.ServerHost, opts.ServerPort)
	fmt.Fprintf(os.Stderr, "   POST /transcribe - Transcribe audio\n")
	fmt.Fprintf(os.Stderr, "   GET  /health     - Health check\n")
	fmt.Fprintf(os.Stderr, "\nPress Ctrl+C to stop\n")

	// Handle shutdown
	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Start()
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		fmt.Fprintln(os.Stderr, "\n⚠️  Shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownDone := make(chan error, 1)
		go func() {
			shutdownDone <- srv.Shutdown(shutdownCtx)
		}()
		pool.Shutdown()
		return <-shutdownDone
	}
}
