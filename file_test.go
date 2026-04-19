package minimax

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/GizClaw/minimax-go/internal/protocol"
	"github.com/GizClaw/minimax-go/internal/transport"
)

func TestFileUpload(t *testing.T) {
	t.Parallel()

	t.Run("success uploads multipart and maps response", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}

			if r.URL.Path != defaultFileUploadPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultFileUploadPath)
			}

			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;") {
				t.Fatalf("content-type = %q, want multipart/form-data", r.Header.Get("Content-Type"))
			}

			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatalf("ParseMultipartForm() error = %v", err)
			}

			if got := r.FormValue("purpose"); got != "voice_clone" {
				t.Fatalf("purpose field = %q, want voice_clone", got)
			}

			file, header, err := r.FormFile(defaultFileFieldName)
			if err != nil {
				t.Fatalf("FormFile() error = %v", err)
			}
			defer file.Close()

			if header.Filename != "demo.wav" {
				t.Fatalf("header.Filename = %q, want demo.wav", header.Filename)
			}

			if got := header.Header.Get("Content-Type"); got != "audio/wav" {
				t.Fatalf("file content type = %q, want audio/wav", got)
			}

			content, err := io.ReadAll(file)
			if err != nil {
				t.Fatalf("ReadAll(file) error = %v", err)
			}

			if string(content) != "hello file" {
				t.Fatalf("file content = %q, want hello file", string(content))
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"data":{"file_id":"file_123","file_url":"https://cdn.example.com/file_123","file_name":"demo.wav","content_type":"audio/wav","size":10}}`))
		}))
		defer srv.Close()

		client, err := NewClient(Config{
			BaseURL:    srv.URL,
			HTTPClient: srv.Client(),
			Retry: transport.RetryConfig{
				MaxAttempts: 1,
			},
		})
		if err != nil {
			t.Fatalf("NewClient() error = %v, want nil", err)
		}

		response, err := client.File.Upload(context.Background(), FileUploadRequest{
			Purpose:     "voice_clone",
			FileName:    "demo.wav",
			ContentType: "audio/wav",
			Data:        []byte("hello file"),
		})
		if err != nil {
			t.Fatalf("Upload() error = %v, want nil", err)
		}

		if response.FileID != "file_123" {
			t.Fatalf("response.FileID = %q, want file_123", response.FileID)
		}

		if response.FileURL != "https://cdn.example.com/file_123" {
			t.Fatalf("response.FileURL = %q, want https://cdn.example.com/file_123", response.FileURL)
		}

		if !response.Uploaded {
			t.Fatal("response.Uploaded = false, want true")
		}

		if response.Meta.FileName != "demo.wav" {
			t.Fatalf("response.Meta.FileName = %q, want demo.wav", response.Meta.FileName)
		}

		if response.Meta.ContentType != "audio/wav" {
			t.Fatalf("response.Meta.ContentType = %q, want audio/wav", response.Meta.ContentType)
		}

		if response.Meta.Size != 10 {
			t.Fatalf("response.Meta.Size = %d, want 10", response.Meta.Size)
		}
	})

	t.Run("numeric file_id in response is normalized to string", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"data":{"file_id":123456789,"file_url":"https://cdn.example.com/file_123456789","file_name":"demo.wav","content_type":"audio/wav","size":10}}`))
		}))
		defer srv.Close()

		client, err := NewClient(Config{
			BaseURL:    srv.URL,
			HTTPClient: srv.Client(),
			Retry: transport.RetryConfig{
				MaxAttempts: 1,
			},
		})
		if err != nil {
			t.Fatalf("NewClient() error = %v, want nil", err)
		}

		response, err := client.File.Upload(context.Background(), FileUploadRequest{
			Purpose:     "voice_clone",
			FileName:    "demo.wav",
			ContentType: "audio/wav",
			Data:        []byte("hello file"),
		})
		if err != nil {
			t.Fatalf("Upload() error = %v, want nil", err)
		}

		if response.FileID != "123456789" {
			t.Fatalf("response.FileID = %q, want 123456789", response.FileID)
		}
	})

	t.Run("empty file name fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		_, err = client.File.Upload(context.Background(), FileUploadRequest{
			Data: []byte("hello"),
		})
		if err == nil {
			t.Fatal("Upload() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "file name is empty") {
			t.Fatalf("Upload() error = %v, want file name validation error", err)
		}
	})

	t.Run("empty data fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		_, err = client.File.Upload(context.Background(), FileUploadRequest{
			FileName: "demo.wav",
		})
		if err == nil {
			t.Fatal("Upload() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "data is empty") {
			t.Fatalf("Upload() error = %v, want data validation error", err)
		}
	})

	t.Run("data size equal max limit succeeds", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"data":{"file_id":"file_limit_equal"}}`))
		}))
		defer srv.Close()

		client, err := NewClient(Config{
			BaseURL:    srv.URL,
			HTTPClient: srv.Client(),
			Retry: transport.RetryConfig{
				MaxAttempts: 1,
			},
		})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		client.File.maxUploadBytes = 5

		response, err := client.File.Upload(context.Background(), FileUploadRequest{
			FileName: "demo.txt",
			Data:     []byte("hello"),
		})
		if err != nil {
			t.Fatalf("Upload() error = %v, want nil", err)
		}

		if response.FileID != "file_limit_equal" {
			t.Fatalf("response.FileID = %q, want file_limit_equal", response.FileID)
		}
	})

	t.Run("data size exceeds max limit fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		client.File.maxUploadBytes = 4

		_, err = client.File.Upload(context.Background(), FileUploadRequest{
			FileName: "demo.txt",
			Data:     []byte("hello"),
		})
		if err == nil {
			t.Fatal("Upload() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "exceeds max size") {
			t.Fatalf("Upload() error = %v, want max size validation error", err)
		}
	})

	t.Run("invalid content type fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		_, err = client.File.Upload(context.Background(), FileUploadRequest{
			FileName:    "demo.wav",
			ContentType: "invalid-content-type",
			Data:        []byte("hello"),
		})
		if err == nil {
			t.Fatal("Upload() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "content type is invalid") {
			t.Fatalf("Upload() error = %v, want content type validation error", err)
		}
	})

	t.Run("http 5xx returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"temporary unavailable"}`))
		}))
		defer srv.Close()

		client, err := NewClient(Config{
			BaseURL:    srv.URL,
			HTTPClient: srv.Client(),
			Retry: transport.RetryConfig{
				MaxAttempts: 1,
			},
		})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		_, err = client.File.Upload(context.Background(), FileUploadRequest{
			FileName: "demo.wav",
			Data:     []byte("hello"),
		})
		if err == nil {
			t.Fatal("Upload() error = nil, want non-nil")
		}

		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("Upload() error type = %T, want *protocol.APIError", err)
		}

		if apiErr.HTTPStatus != http.StatusServiceUnavailable {
			t.Fatalf("apiErr.HTTPStatus = %d, want %d", apiErr.HTTPStatus, http.StatusServiceUnavailable)
		}
	})

	t.Run("base_resp non-zero returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":2301,"status_msg":"invalid file"}}`))
		}))
		defer srv.Close()

		client, err := NewClient(Config{
			BaseURL:    srv.URL,
			HTTPClient: srv.Client(),
			Retry: transport.RetryConfig{
				MaxAttempts: 1,
			},
		})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		_, err = client.File.Upload(context.Background(), FileUploadRequest{
			FileName: "demo.wav",
			Data:     []byte("hello"),
		})
		if err == nil {
			t.Fatal("Upload() error = nil, want non-nil")
		}

		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("Upload() error type = %T, want *protocol.APIError", err)
		}

		if apiErr.StatusCode != 2301 || apiErr.StatusMsg != "invalid file" {
			t.Fatalf("apiErr = %+v, want status_code=2301 status_msg=invalid file", apiErr)
		}
	})

	t.Run("context canceled is preserved", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(120 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"data":{"file_id":"file_123"}}`))
		}))
		defer srv.Close()

		client, err := NewClient(Config{
			BaseURL:    srv.URL,
			HTTPClient: srv.Client(),
			Retry: transport.RetryConfig{
				MaxAttempts: 1,
			},
		})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = client.File.Upload(ctx, FileUploadRequest{
			FileName: "demo.wav",
			Data:     []byte("hello"),
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Upload() error = %v, want context canceled", err)
		}
	})
}
