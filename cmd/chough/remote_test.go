package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestTranscribeRemoteAcceptsOldAndNewResponses(t *testing.T) {
	for _, test := range []struct {
		name          string
		response      string
		wantChunks    int
		wantDuration  float64
		wantDurations int
		wantLogProbs  int
		wantText      string
	}{
		{
			name:         "old chunk response",
			response:     `{"success":true,"duration_seconds":2,"text":"hello","chunks":[{"start_time":0,"end_time":2,"text":"hello","timestamps":[0],"tokens":["hello"]}]}`,
			wantChunks:   1,
			wantDuration: 2,
			wantText:     "hello",
		},
		{
			name:          "new additive response",
			response:      `{"success":true,"duration_seconds":2,"text":"hello","chunks":[{"start_time":0,"end_time":2,"text":"hello","timestamps":[0],"tokens":["hello"],"durations":[0.4],"log_probs":[-0.2]}],"future_field":"ignored"}`,
			wantChunks:    1,
			wantDuration:  2,
			wantDurations: 1,
			wantLogProbs:  1,
			wantText:      "hello",
		},
		{
			name:         "text fallback",
			response:     `{"success":true,"duration_seconds":2,"text":"fallback text"}`,
			wantChunks:   1,
			wantDuration: 2,
			wantText:     "fallback text",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, test.response)
			}))
			defer server.Close()
			audioFile := writeRemoteTestAudio(t)

			chunks, duration, err := transcribeRemote(context.Background(), server.URL, audioFile, 60)
			if err != nil {
				t.Fatalf("transcribeRemote() error = %v", err)
			}
			if len(chunks) != test.wantChunks || duration != test.wantDuration {
				t.Fatalf("result = %d chunks, duration %v", len(chunks), duration)
			}
			if chunks[0].Text != test.wantText {
				t.Fatalf("text = %q, want %q", chunks[0].Text, test.wantText)
			}
			if len(chunks[0].Durations) != test.wantDurations || len(chunks[0].LogProbs) != test.wantLogProbs {
				t.Fatalf("optional token fields = durations %v, log_probs %v", chunks[0].Durations, chunks[0].LogProbs)
			}
		})
	}
}

func TestTranscribeRemoteStreamsMultipartBody(t *testing.T) {
	contentLength := make(chan int64, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		contentLength <- request.ContentLength
		if err := request.ParseMultipartForm(1024); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if request.FormValue("chunk_size") != "30" || request.FormValue("format") != "json" {
			http.Error(w, "invalid multipart fields", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"success":true,"text":"ok"}`)
	}))
	defer server.Close()

	if _, _, err := transcribeRemote(context.Background(), server.URL, writeRemoteTestAudio(t), 30); err != nil {
		t.Fatalf("transcribeRemote() error = %v", err)
	}
	if got := <-contentLength; got != -1 {
		t.Fatalf("Content-Length = %d, want -1 for streaming upload", got)
	}
}

func TestTranscribeRemoteHonorsCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	}))
	defer server.Close()
	defer close(release)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-started
		cancel()
	}()
	_, _, err := transcribeRemote(ctx, server.URL, writeRemoteTestAudio(t), 60)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("transcribeRemote() error = %v, want context.Canceled", err)
	}
}

func writeRemoteTestAudio(t *testing.T) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "audio-*.wav")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("audio"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return file.Name()
}
