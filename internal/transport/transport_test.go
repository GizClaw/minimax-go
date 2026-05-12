package transport

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GizClaw/minimax-go/internal/protocol"
)

func TestDoJSON(t *testing.T) {
	t.Parallel()

	t.Run("success", testDoJSONSuccess)
	t.Run("base_resp error", testDoJSONBaseRespError)
	t.Run("retry eventually success", testDoJSONRetryEventuallySuccess)
	t.Run("retry exhausted returns last retryable error", testDoJSONRetryExhausted)
	t.Run("request headers override default headers", testDoJSONHeaderOverride)
	t.Run("response metadata", testDoJSONResponseMeta)
	t.Run("timeout canceled by context", testDoJSONContextTimeout)
}

func testDoJSONSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"value":"done"}`))
	}))
	defer srv.Close()

	client, err := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}

	var out struct {
		Value string `json:"value"`
	}

	err = client.DoJSON(context.Background(), JSONRequest{
		Method: http.MethodPost,
		Path:   "/json",
		Body:   map[string]string{"text": "hello"},
	}, &out)
	if err != nil {
		t.Fatalf("DoJSON() error = %v, want nil", err)
	}

	if out.Value != "done" {
		t.Fatalf("out.Value = %q, want done", out.Value)
	}
}

func testDoJSONBaseRespError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Trace-ID", "trace-error-header")
		_, _ = w.Write([]byte(`{"base_resp":{"status_code":2301,"status_msg":"invalid model"},"trace_id":"trace-error-body"}`))
	}))
	defer srv.Close()

	client, err := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}

	err = client.DoJSON(context.Background(), JSONRequest{Path: "/json"}, nil)
	if err == nil {
		t.Fatal("DoJSON() error = nil, want non-nil")
	}

	var apiErr *protocol.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("DoJSON() error type = %T, want *protocol.APIError", err)
	}

	if apiErr.StatusCode != 2301 {
		t.Fatalf("apiErr.StatusCode = %d, want 2301", apiErr.StatusCode)
	}

	if apiErr.TraceID != "trace-error-header" {
		t.Fatalf("apiErr.TraceID = %q, want trace-error-header", apiErr.TraceID)
	}
}

func testDoJSONResponseMeta(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", "req-header")
		w.Header().Set("X-Trace-ID", "trace-header")
		_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"request_id":"req-body","trace_id":"trace-body","value":"done"}`))
	}))
	defer srv.Close()

	client, err := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}

	var out struct {
		Value string `json:"value"`
	}

	meta, err := client.DoJSONWithMeta(context.Background(), JSONRequest{Path: "/json"}, &out)
	if err != nil {
		t.Fatalf("DoJSONWithMeta() error = %v, want nil", err)
	}

	if meta.RequestID != "req-header" || meta.TraceID != "trace-header" {
		t.Fatalf("meta = %+v, want request_id=req-header trace_id=trace-header", meta)
	}

	if meta.HTTPStatus != http.StatusOK {
		t.Fatalf("meta.HTTPStatus = %d, want %d", meta.HTTPStatus, http.StatusOK)
	}
}

func testDoJSONRetryEventuallySuccess(t *testing.T) {
	t.Parallel()

	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&attempts, 1)
		if current < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"temporary"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"value":"ok"}`))
	}))
	defer srv.Close()

	client, err := New(Config{
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
		Retry: RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   time.Millisecond,
			MaxDelay:    time.Millisecond,
			Sleep: func(context.Context, time.Duration) error {
				return nil
			},
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}

	err = client.DoJSON(context.Background(), JSONRequest{Path: "/json"}, nil)
	if err != nil {
		t.Fatalf("DoJSON() error = %v, want nil", err)
	}

	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("attempts = %d, want 3", got)
	}
}

func testDoJSONRetryExhausted(t *testing.T) {
	t.Parallel()

	var attempts int32
	var sleepCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"still unavailable"}`))
	}))
	defer srv.Close()

	client, err := New(Config{
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
		Retry: RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   time.Millisecond,
			MaxDelay:    time.Millisecond,
			Sleep: func(context.Context, time.Duration) error {
				atomic.AddInt32(&sleepCalls, 1)
				return nil
			},
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}

	err = client.DoJSON(context.Background(), JSONRequest{Path: "/json"}, nil)
	if err == nil {
		t.Fatal("DoJSON() error = nil, want non-nil")
	}

	var apiErr *protocol.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("DoJSON() error type = %T, want *protocol.APIError", err)
	}

	if apiErr.HTTPStatus != http.StatusServiceUnavailable {
		t.Fatalf("apiErr.HTTPStatus = %d, want %d", apiErr.HTTPStatus, http.StatusServiceUnavailable)
	}

	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("attempts = %d, want 3", got)
	}

	if got := atomic.LoadInt32(&sleepCalls); got != 2 {
		t.Fatalf("sleepCalls = %d, want 2", got)
	}
}

func testDoJSONHeaderOverride(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorizationValues := r.Header.Values("Authorization")
		if len(authorizationValues) != 1 {
			t.Fatalf("Authorization values = %v, want single value", authorizationValues)
		}

		if authorizationValues[0] != "Bearer request-token" {
			t.Fatalf("Authorization = %q, want %q", authorizationValues[0], "Bearer request-token")
		}

		contentTypeValues := r.Header.Values("Content-Type")
		if len(contentTypeValues) != 1 {
			t.Fatalf("Content-Type values = %v, want single value", contentTypeValues)
		}

		if contentTypeValues[0] != "application/custom+json" {
			t.Fatalf("Content-Type = %q, want %q", contentTypeValues[0], "application/custom+json")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"}}`))
	}))
	defer srv.Close()

	client, err := New(Config{
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
		DefaultHeaders: http.Header{
			"Authorization": []string{"Bearer default-token"},
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}

	err = client.DoJSON(context.Background(), JSONRequest{
		Path: "/json",
		Headers: http.Header{
			"Authorization": []string{"Bearer request-token"},
			"Content-Type":  []string{"application/custom+json"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("DoJSON() error = %v, want nil", err)
	}
}

func testDoJSONContextTimeout(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"}}`))
	}))
	defer srv.Close()

	client, err := New(Config{
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
		Retry: RetryConfig{
			MaxAttempts: 1,
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err = client.DoJSON(ctx, JSONRequest{Path: "/json"}, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("DoJSON() error = %v, want context deadline exceeded", err)
	}
}

func TestOpenStream(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("data: hello\n\n"))
		}))
		defer srv.Close()

		client, err := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
		if err != nil {
			t.Fatalf("New() error = %v, want nil", err)
		}

		body, err := client.OpenStream(context.Background(), StreamRequest{Path: "/stream"})
		if err != nil {
			t.Fatalf("OpenStream() error = %v, want nil", err)
		}
		defer body.Close()

		data, err := io.ReadAll(body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v, want nil", err)
		}

		if string(data) != "data: hello\n\n" {
			t.Fatalf("stream data = %q, want %q", string(data), "data: hello\n\n")
		}
	})

	t.Run("http 200 with base_resp error should fail", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":2301,"status_msg":"stream failed"}}`))
		}))
		defer srv.Close()

		client, err := New(Config{
			BaseURL:    srv.URL,
			HTTPClient: srv.Client(),
			Retry: RetryConfig{
				MaxAttempts: 1,
			},
		})
		if err != nil {
			t.Fatalf("New() error = %v, want nil", err)
		}

		_, err = client.OpenStream(context.Background(), StreamRequest{Path: "/stream"})
		if err == nil {
			t.Fatal("OpenStream() error = nil, want non-nil")
		}

		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("OpenStream() error type = %T, want *protocol.APIError", err)
		}

		if apiErr.StatusCode != 2301 {
			t.Fatalf("apiErr.StatusCode = %d, want 2301", apiErr.StatusCode)
		}
	})

	t.Run("unexpected non-stream content type", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"}}`))
		}))
		defer srv.Close()

		client, err := New(Config{
			BaseURL:    srv.URL,
			HTTPClient: srv.Client(),
			Retry: RetryConfig{
				MaxAttempts: 1,
			},
		})
		if err != nil {
			t.Fatalf("New() error = %v, want nil", err)
		}

		_, err = client.OpenStream(context.Background(), StreamRequest{Path: "/stream"})
		if err == nil {
			t.Fatal("OpenStream() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "unexpected stream content type") {
			t.Fatalf("OpenStream() error = %v, want unexpected stream content type", err)
		}
	})
}

func TestUpload(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm() error = %v", err)
		}

		if got := r.FormValue("model"); got != "tts-v1" {
			t.Fatalf("model field = %q, want tts-v1", got)
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile() error = %v", err)
		}
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("ReadAll(file) error = %v", err)
		}

		if string(content) != "hello" {
			t.Fatalf("file content = %q, want hello", string(content))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"uploaded":true}`))
	}))
	defer srv.Close()

	client, err := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}

	var out struct {
		Uploaded bool `json:"uploaded"`
	}

	err = client.Upload(context.Background(), UploadRequest{
		Path:      "/upload",
		FileField: "file",
		FileName:  "hello.txt",
		FileData:  []byte("hello"),
		Fields: map[string]string{
			"model": "tts-v1",
		},
	}, &out)
	if err != nil {
		t.Fatalf("Upload() error = %v, want nil", err)
	}

	if !out.Uploaded {
		t.Fatalf("out.Uploaded = false, want true")
	}
}

func TestUploadRequiresFileInfo(t *testing.T) {
	t.Parallel()

	client, err := New(Config{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = client.Upload(context.Background(), UploadRequest{}, nil)
	if err == nil {
		t.Fatal("Upload() error = nil, want non-nil")
	}
}

func TestUploadHeaderBehavior(t *testing.T) {
	t.Parallel()

	t.Run("api key injects authorization header", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Authorization"); got != "Bearer upload-token" {
				t.Fatalf("Authorization = %q, want Bearer upload-token", got)
			}

			contentType := r.Header.Get("Content-Type")
			if !strings.HasPrefix(contentType, "multipart/form-data;") {
				t.Fatalf("Content-Type = %q, want multipart/form-data", contentType)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"}}`))
		}))
		defer srv.Close()

		client, err := New(Config{
			BaseURL:    srv.URL,
			APIKey:     "upload-token",
			HTTPClient: srv.Client(),
		})
		if err != nil {
			t.Fatalf("New() error = %v, want nil", err)
		}

		err = client.Upload(context.Background(), UploadRequest{
			Path:      "/upload",
			FileField: "file",
			FileName:  "demo.txt",
			FileData:  []byte("hello"),
		}, nil)
		if err != nil {
			t.Fatalf("Upload() error = %v, want nil", err)
		}
	})

	t.Run("request headers override defaults and keep multipart content type", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Authorization"); got != "Bearer request-token" {
				t.Fatalf("Authorization = %q, want Bearer request-token", got)
			}

			if got := r.Header.Get("X-Trace-ID"); got != "request-trace" {
				t.Fatalf("X-Trace-ID = %q, want request-trace", got)
			}

			contentType := r.Header.Get("Content-Type")
			mediaType, params, err := mime.ParseMediaType(contentType)
			if err != nil {
				t.Fatalf("ParseMediaType(Content-Type) error = %v", err)
			}

			if mediaType != "multipart/form-data" {
				t.Fatalf("mediaType = %q, want multipart/form-data", mediaType)
			}

			if strings.TrimSpace(params["boundary"]) == "" {
				t.Fatalf("multipart boundary is empty in Content-Type %q", contentType)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"}}`))
		}))
		defer srv.Close()

		client, err := New(Config{
			BaseURL:    srv.URL,
			HTTPClient: srv.Client(),
			DefaultHeaders: http.Header{
				"Authorization": []string{"Bearer default-token"},
				"X-Trace-ID":    []string{"default-trace"},
			},
		})
		if err != nil {
			t.Fatalf("New() error = %v, want nil", err)
		}

		err = client.Upload(context.Background(), UploadRequest{
			Path:      "/upload",
			FileField: "file",
			FileName:  "demo.txt",
			FileData:  []byte("hello"),
			Headers: http.Header{
				"Authorization": []string{"Bearer request-token"},
				"X-Trace-ID":    []string{"request-trace"},
				"Content-Type":  []string{"application/json"},
			},
		}, nil)
		if err != nil {
			t.Fatalf("Upload() error = %v, want nil", err)
		}
	})
}

// Compile-time guard: keep multipart dependency linked in tests.
var _ = multipart.ErrMessageTooLarge
