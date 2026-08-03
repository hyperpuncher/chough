package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"strings"
	"testing"
)

type staticResolver map[string][]netip.Addr

func (r staticResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	return r[host], nil
}

func TestURLPolicyBlocksUnsafeDestinations(t *testing.T) {
	resolver := staticResolver{
		"public.example":  {netip.MustParseAddr("8.8.8.8")},
		"private.example": {netip.MustParseAddr("192.168.1.10")},
	}
	policy := newURLPolicy(false, resolver)

	for _, rawURL := range []string{
		"file:///tmp/audio.wav",
		"http://user:password@public.example/audio.wav",
		"http://127.0.0.1/audio.wav",
		"http://169.254.169.254/audio.wav",
		"http://private.example/audio.wav",
	} {
		if _, err := policy.validate(context.Background(), rawURL); err == nil {
			t.Errorf("validate(%q) error = nil, want blocked URL", rawURL)
		}
	}
	if _, err := policy.validate(context.Background(), "https://public.example/audio.wav"); err != nil {
		t.Fatalf("public URL rejected: %v", err)
	}
}

func TestURLPolicyCanAllowPrivateDestinations(t *testing.T) {
	policy := newURLPolicy(true, nil)
	if _, err := policy.validate(context.Background(), "http://127.0.0.1/audio.wav"); err != nil {
		t.Fatalf("private URL rejected with opt-in: %v", err)
	}
}

func TestDownloadRejectsRedirectToPrivateDestination(t *testing.T) {
	policy := newURLPolicy(false, staticResolver{
		"public.example":  {netip.MustParseAddr("8.8.8.8")},
		"private.example": {netip.MustParseAddr("10.0.0.2")},
	})
	client := newDownloadClient(policy)
	request, err := http.NewRequest(http.MethodGet, "http://private.example/audio.wav", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(request, []*http.Request{{}}); err == nil {
		t.Fatal("CheckRedirect() error = nil, want private redirect rejection")
	}
}

func TestDownloadEnforcesResponseLimit(t *testing.T) {
	for _, tt := range []struct {
		name    string
		size    int
		wantErr bool
	}{
		{name: "at limit", size: int(megabyte)},
		{name: "above limit", size: int(megabyte) + 1, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(strings.Repeat("a", tt.size)))
			}))
			defer upstream.Close()

			policy := newURLPolicy(true, nil)
			server := &Server{
				options:        ServerOptions{MaxUploadMB: 1},
				urlPolicy:      policy,
				downloadClient: newDownloadClient(policy),
			}
			path, err := server.downloadFromURL(context.Background(), upstream.URL)
			if tt.wantErr {
				if err == nil {
					os.Remove(path)
					t.Fatal("downloadFromURL() error = nil, want size error")
				}
				return
			}
			if err != nil {
				t.Fatalf("downloadFromURL() error = %v", err)
			}
			defer os.Remove(path)
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Size() != int64(tt.size) {
				t.Fatalf("downloaded size = %d, want %d", info.Size(), tt.size)
			}
		})
	}
}
