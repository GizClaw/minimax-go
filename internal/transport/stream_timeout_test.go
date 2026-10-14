package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"
)

func TestDefaultStreamDurationUsesContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	body, err := client.OpenStream(ctx, StreamRequest{Path: "/stream"})
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	read := make(chan error, 1)
	go func() { _, err := io.ReadAll(body); read <- err }()
	cancel()
	select {
	case err := <-read:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled stream: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("stream cancellation did not unblock the body")
	}
}

func TestStreamBodyCanOutliveDefaultJSONTimeout(t *testing.T) {
	previous := http.DefaultTransport
	defer func() { http.DefaultTransport = previous }()
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "context_governed", true: "explicit_timeout"}[explicit], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				transport := streamTimeoutRoundTripper(func(r *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: &delayedStreamBody{ctx: r.Context()}}, nil
				})
				http.DefaultTransport = transport
				config := Config{BaseURL: "https://example.invalid"}
				if explicit {
					config.HTTPClient = &http.Client{Transport: transport, Timeout: 17 * time.Second}
				}
				client, err := New(config)
				if err != nil {
					t.Fatal(err)
				}
				body, err := client.OpenStream(t.Context(), StreamRequest{Path: "/stream"})
				if err != nil {
					t.Fatal(err)
				}
				defer body.Close()
				start := time.Now()
				data, err := io.ReadAll(body)
				if explicit {
					if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) != 17*time.Second {
						t.Fatalf("explicit timeout elapsed=%s err=%v", time.Since(start), err)
					}
				} else if err != nil || string(data) != "late audio" || time.Since(start) != 31*time.Second {
					t.Fatalf("long stream elapsed=%s data=%q err=%v", time.Since(start), data, err)
				}
			})
		})
	}
}

type streamTimeoutRoundTripper func(*http.Request) (*http.Response, error)

func (f streamTimeoutRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type delayedStreamBody struct{ ctx context.Context }

func (b *delayedStreamBody) Read(p []byte) (int, error) {
	select {
	case <-time.After(31 * time.Second):
		return copy(p, "late audio"), io.EOF
	case <-b.ctx.Done():
		return 0, b.ctx.Err()
	}
}

func (*delayedStreamBody) Close() error { return nil }

func TestConfiguredHTTPClientTimeoutRemainsAuthoritative(t *testing.T) {
	httpClient := &http.Client{Timeout: 17 * time.Second}
	client, err := New(Config{BaseURL: "https://example.invalid", HTTPClient: httpClient})
	if err != nil {
		t.Fatal(err)
	}
	if client.httpClient != httpClient || client.streamHTTPClient != httpClient {
		t.Fatal("explicit HTTP client was replaced")
	}
}
