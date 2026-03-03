package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hyperpuncher/chough/internal/server"
	"github.com/hyperpuncher/chough/internal/worker"
)

func runServer(opts *cliOptions) error {
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
	serverOpts := &server.ServerOptions{
		Host:         opts.ServerHost,
		Port:         opts.ServerPort,
		MaxUploadMB:  int64(opts.MaxUploadMB),
		Workers:      opts.Workers,
		MaxQueueSize: 10,
	}
	pool := worker.NewPool(opts.Workers, 10, recognizer)
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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return err
	case <-sigChan:
		fmt.Fprintln(os.Stderr, "\n⚠️  Shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}
