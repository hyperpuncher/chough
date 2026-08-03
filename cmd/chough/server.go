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

func runServer(ctx context.Context, opts cliOptions) error {
	if err := server.ValidateOptions(opts.ServerOptions); err != nil {
		return fmt.Errorf("invalid server configuration: %w", err)
	}

	recognizer, err := loadRecognizer(ctx)
	if err != nil {
		return err
	}
	defer recognizer.Close()

	service := transcribe.NewService(recognizer)
	pool := worker.NewPool(opts.ServerOptions.Workers, opts.ServerOptions.MaxQueueSize, service)
	defer pool.Shutdown()

	srv := server.NewServer(opts.ServerOptions, pool, version)

	fmt.Fprintf(os.Stderr, "🚀 Server running on http://%s:%d\n", opts.ServerOptions.Host, opts.ServerOptions.Port)
	fmt.Fprintf(os.Stderr, "   POST /transcribe - Transcribe audio\n")
	fmt.Fprintf(os.Stderr, "   GET  /health     - Health check\n")
	fmt.Fprintf(os.Stderr, "\nPress Ctrl+C to stop\n")

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
