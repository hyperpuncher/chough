package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

func TestGetModelPathContextRejectsInvalidExplicitModel(t *testing.T) {
	t.Setenv("CHOUGH_MODEL", "  "+t.TempDir()+"  ")
	_, err := GetModelPathContext(context.Background(), nil)
	if err == nil {
		t.Fatal("GetModelPathContext() unexpectedly succeeded")
	}
	for _, name := range requiredModelFiles {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("error %q does not name missing file %q", err, name)
		}
	}
}

func TestGetModelPathContextUsesValidExplicitModel(t *testing.T) {
	modelDir := t.TempDir()
	writeRequiredModelFiles(t, modelDir)
	t.Setenv("CHOUGH_MODEL", "  "+modelDir+"  ")

	got, err := GetModelPathContext(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetModelPathContext() error = %v", err)
	}
	want, err := filepath.Abs(modelDir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("model path = %q, want %q", got, want)
	}
}

func TestResolveModelInstallsValidArchiveAtomically(t *testing.T) {
	archive := readFixture(t, "valid.tar.bz2")
	server, definition, requests := archiveServer(t, http.StatusOK, archive, 0)
	defer server.Close()
	definition.url = server.URL
	modelDir := filepath.Join(t.TempDir(), definition.name)
	var stages []string

	got, err := resolveModel(context.Background(), definition, modelDir, server.Client(), func(progress Progress) {
		stages = append(stages, progress.Stage)
	})
	if err != nil {
		t.Fatalf("resolveModel() error = %v", err)
	}
	if got != modelDir || len(missingModelFiles(modelDir)) != 0 {
		t.Fatalf("installed model is invalid at %q", got)
	}
	if requests.Load() != 1 {
		t.Fatalf("download requests = %d, want 1", requests.Load())
	}
	if !contains(stages, "download") || !contains(stages, "extract") || !contains(stages, "ready") {
		t.Fatalf("progress stages = %v", stages)
	}
	entries, err := os.ReadDir(filepath.Dir(modelDir))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".chough-model-") {
			t.Fatalf("temporary installation state remains: %s", entry.Name())
		}
	}
}

func TestResolveModelDownloadsOnceConcurrently(t *testing.T) {
	archive := readFixture(t, "valid.tar.bz2")
	server, definition, requests := archiveServer(t, http.StatusOK, archive, 30*time.Millisecond)
	defer server.Close()
	definition.url = server.URL
	modelDir := filepath.Join(t.TempDir(), definition.name)

	const callers = 4
	var wait sync.WaitGroup
	errors := make(chan error, callers)
	wait.Add(callers)
	for range callers {
		go func() {
			defer wait.Done()
			_, err := resolveModel(context.Background(), definition, modelDir, server.Client(), nil)
			errors <- err
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent resolveModel() error = %v", err)
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("download requests = %d, want 1", requests.Load())
	}
}

func TestResolveModelLeavesNoFinalDirectoryOnInstallFailure(t *testing.T) {
	for _, test := range []struct {
		name    string
		fixture string
		want    string
		mutate  func(*modelDefinition)
	}{
		{name: "checksum mismatch", fixture: "valid.tar.bz2", want: "checksum mismatch", mutate: func(definition *modelDefinition) { definition.sha256 = strings.Repeat("0", 64) }},
		{name: "incomplete model", fixture: "incomplete.tar.bz2", want: "incomplete"},
		{name: "interrupted extraction", fixture: "partial-then-invalid.tar.bz2", want: "unsupported link type"},
	} {
		t.Run(test.name, func(t *testing.T) {
			archive := readFixture(t, test.fixture)
			server, definition, _ := archiveServer(t, http.StatusOK, archive, 0)
			defer server.Close()
			definition.url = server.URL
			if test.mutate != nil {
				test.mutate(&definition)
			}
			modelDir := filepath.Join(t.TempDir(), definition.name)
			if err := os.Mkdir(modelDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(modelDir, "partial"), []byte("partial"), 0o644); err != nil {
				t.Fatal(err)
			}

			if _, err := resolveModel(context.Background(), definition, modelDir, server.Client(), nil); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("resolveModel() error = %v, want containing %q", err, test.want)
			}
			if _, err := os.Stat(modelDir); !os.IsNotExist(err) {
				t.Fatalf("final model directory exists after failure: err = %v", err)
			}
		})
	}
}

func TestResolveModelReportsHTTPFailure(t *testing.T) {
	server, definition, _ := archiveServer(t, http.StatusBadGateway, []byte("upstream unavailable"), 0)
	defer server.Close()
	definition.url = server.URL
	modelDir := filepath.Join(t.TempDir(), definition.name)

	_, err := resolveModel(context.Background(), definition, modelDir, server.Client(), nil)
	if err == nil || !strings.Contains(err.Error(), "502 Bad Gateway") {
		t.Fatalf("resolveModel() error = %v, want HTTP status", err)
	}
}

func TestResolveModelHonorsHTTPTimeout(t *testing.T) {
	archive := readFixture(t, "valid.tar.bz2")
	server, definition, _ := archiveServer(t, http.StatusOK, archive, 100*time.Millisecond)
	defer server.Close()
	definition.url = server.URL
	client := server.Client()
	client.Timeout = 10 * time.Millisecond
	modelDir := filepath.Join(t.TempDir(), definition.name)

	_, err := resolveModel(context.Background(), definition, modelDir, client, nil)
	if err == nil || !strings.Contains(err.Error(), "Client.Timeout") {
		t.Fatalf("resolveModel() error = %v, want client timeout", err)
	}
}

func TestResolveModelHonorsCancellationWhileWaitingForLock(t *testing.T) {
	modelDir := filepath.Join(t.TempDir(), "test-model")
	lock := flockForTest(t, modelDir+".lock")
	defer lock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := resolveModel(ctx, modelDefinition{name: "test-model"}, modelDir, http.DefaultClient, nil)
	if err == nil || !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("resolveModel() error = %v, want deadline exceeded", err)
	}
}

func archiveServer(t *testing.T, status int, archive []byte, delay time.Duration) (*httptest.Server, modelDefinition, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		if delay > 0 {
			time.Sleep(delay)
		}
		w.WriteHeader(status)
		_, _ = w.Write(archive)
	}))
	hash := sha256.Sum256(archive)
	return server, modelDefinition{
		name:   "test-model",
		sha256: hex.EncodeToString(hash[:]),
		size:   int64(len(archive)),
	}, &requests
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeRequiredModelFiles(t *testing.T, modelDir string) {
	t.Helper()
	for _, name := range requiredModelFiles {
		if err := os.WriteFile(filepath.Join(modelDir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func flockForTest(t *testing.T, path string) func() {
	t.Helper()
	lock := flock.New(path)
	locked, err := lock.TryLock()
	if err != nil || !locked {
		t.Fatalf("lock test cache: locked=%v err=%v", locked, err)
	}
	return func() {
		if err := lock.Unlock(); err != nil {
			t.Errorf("unlock test cache: %v", err)
		}
	}
}
