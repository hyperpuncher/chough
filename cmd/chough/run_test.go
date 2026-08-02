package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestWriteAndCloseOutputReportsCloseError(t *testing.T) {
	closeErr := errors.New("close failed")
	err := writeAndCloseOutput(&bytes.Buffer{}, func() error { return closeErr }, "text", nil, 0)
	if !errors.Is(err, closeErr) || !strings.Contains(err.Error(), "error closing output") {
		t.Fatalf("writeAndCloseOutput() error = %v", err)
	}
}

func TestWriteAndCloseOutputReportsWriteAndCloseErrors(t *testing.T) {
	writeErr := errors.New("write failed")
	closeErr := errors.New("close failed")
	err := writeAndCloseOutput(errorWriter{err: writeErr}, func() error { return closeErr }, "text", nil, 0)
	if !errors.Is(err, writeErr) || !errors.Is(err, closeErr) {
		t.Fatalf("writeAndCloseOutput() error = %v", err)
	}
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) {
	return 0, w.err
}
