package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestOpenOutputDoesNotOwnStdout(t *testing.T) {
	out, closer, err := openOutput("")
	if err != nil {
		t.Fatalf("openOutput() error = %v", err)
	}
	if out != os.Stdout || closer != nil {
		t.Fatalf("openOutput() = (%v, %v), want stdout with no closer", out, closer)
	}
}

func TestWriteAndCloseOutputReportsCloseError(t *testing.T) {
	closeErr := errors.New("close failed")
	err := writeAndCloseOutput(&bytes.Buffer{}, errorCloser{err: closeErr}, "text", nil, 0)
	if !errors.Is(err, closeErr) || !strings.Contains(err.Error(), "close output") {
		t.Fatalf("writeAndCloseOutput() error = %v", err)
	}
}

func TestWriteAndCloseOutputReportsWriteAndCloseErrors(t *testing.T) {
	writeErr := errors.New("write failed")
	closeErr := errors.New("close failed")
	err := writeAndCloseOutput(errorWriter{err: writeErr}, errorCloser{err: closeErr}, "text", nil, 0)
	if !errors.Is(err, writeErr) || !errors.Is(err, closeErr) {
		t.Fatalf("writeAndCloseOutput() error = %v", err)
	}
}

type errorCloser struct {
	err error
}

func (c errorCloser) Close() error {
	return c.err
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) {
	return 0, w.err
}
