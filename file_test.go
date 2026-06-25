package minimax

import (
	"context"
	"encoding/json"
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

func TestFileList(t *testing.T) {
	t.Parallel()

	t.Run("success lists files by purpose", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Fatalf("method = %s, want GET", r.Method)
			}
			if r.URL.Path != defaultFileListPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultFileListPath)
			}
			if got := r.URL.Query().Get("purpose"); got != "t2a_async_input" {
				t.Fatalf("purpose query = %q, want t2a_async_input", got)
			}

			w.Header().Set("X-Trace-ID", "trace-file-list")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"files":[{"file_id":12345,"bytes":5896337,"created_at":1700469398,"filename":"input.txt","purpose":"t2a_async_input","extra":"kept"}],"base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newFileTestClient(t, srv)
		response, err := client.File.List(context.Background(), FileListRequest{Purpose: " t2a_async_input "})
		if err != nil {
			t.Fatalf("List() error = %v, want nil", err)
		}

		if response.ResponseMeta.TraceID != "trace-file-list" {
			t.Fatalf("TraceID = %q, want trace-file-list", response.ResponseMeta.TraceID)
		}
		if got := len(response.Files); got != 1 {
			t.Fatalf("len(Files) = %d, want 1", got)
		}

		file := response.Files[0]
		if file.FileID != "12345" || file.Bytes != 5896337 || file.CreatedAt != 1700469398 || file.FileName != "input.txt" || file.Purpose != "t2a_async_input" {
			t.Fatalf("file = %+v, want normalized file metadata", file)
		}
		if _, ok := file.Raw["extra"]; !ok {
			t.Fatalf("file.Raw missing extra field: %+v", file.Raw)
		}
	})

	t.Run("empty purpose fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		_, err = client.File.List(context.Background(), FileListRequest{})
		if err == nil {
			t.Fatal("List() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "purpose is empty") {
			t.Fatalf("List() error = %v, want purpose validation error", err)
		}
	})

	t.Run("base_resp non-zero returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":2013,"status_msg":"invalid purpose"}}`))
		}))
		defer srv.Close()

		client := newFileTestClient(t, srv)
		_, err := client.File.List(context.Background(), FileListRequest{Purpose: "voice_clone"})
		assertAPIStatus(t, err, 2013, "invalid purpose")
	})

	t.Run("context canceled is preserved", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = client.File.List(ctx, FileListRequest{Purpose: "voice_clone"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("List() error = %v, want context canceled", err)
		}
	})
}

func TestFileRetrieve(t *testing.T) {
	t.Parallel()

	t.Run("success retrieves file metadata", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Fatalf("method = %s, want GET", r.Method)
			}
			if r.URL.Path != defaultFileRetrievePath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultFileRetrievePath)
			}
			if got := r.URL.Query().Get("file_id"); got != "205258526306433" {
				t.Fatalf("file_id query = %q, want 205258526306433", got)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"file":{"file_id":205258526306433,"bytes":1024,"created_at":1700469398,"filename":"output.mp4","purpose":"video_generation","download_url":"https://cdn.example.com/output.mp4"},"base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newFileTestClient(t, srv)
		response, err := client.File.Retrieve(context.Background(), " 205258526306433 ")
		if err != nil {
			t.Fatalf("Retrieve() error = %v, want nil", err)
		}

		if response.File.FileID != "205258526306433" || response.File.DownloadURL != "https://cdn.example.com/output.mp4" {
			t.Fatalf("response.File = %+v, want normalized file", response.File)
		}
	})

	t.Run("empty file_id fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		_, err = client.File.Retrieve(context.Background(), " ")
		if err == nil {
			t.Fatal("Retrieve() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "file_id is empty") {
			t.Fatalf("Retrieve() error = %v, want file_id validation error", err)
		}
	})

	t.Run("http 5xx returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"temporary unavailable"}`))
		}))
		defer srv.Close()

		client := newFileTestClient(t, srv)
		_, err := client.File.Retrieve(context.Background(), "file_123")
		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("Retrieve() error type = %T, want *protocol.APIError", err)
		}
		if apiErr.HTTPStatus != http.StatusServiceUnavailable {
			t.Fatalf("apiErr.HTTPStatus = %d, want %d", apiErr.HTTPStatus, http.StatusServiceUnavailable)
		}
	})
}

func TestFileDownload(t *testing.T) {
	t.Parallel()

	t.Run("success opens raw file body", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Fatalf("method = %s, want GET", r.Method)
			}
			if r.URL.Path != defaultFileDownloadPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultFileDownloadPath)
			}
			if got := r.URL.Query().Get("file_id"); got != "file_123" {
				t.Fatalf("file_id query = %q, want file_123", got)
			}

			w.Header().Set("Content-Type", "video/mp4")
			w.Header().Set("Content-Length", "11")
			_, _ = w.Write([]byte("hello video"))
		}))
		defer srv.Close()

		client := newFileTestClient(t, srv)
		response, err := client.File.Download(context.Background(), "file_123")
		if err != nil {
			t.Fatalf("Download() error = %v, want nil", err)
		}
		defer response.Body.Close()

		if response.ContentType != "video/mp4" {
			t.Fatalf("ContentType = %q, want video/mp4", response.ContentType)
		}
		if response.ContentLength != 11 {
			t.Fatalf("ContentLength = %d, want 11", response.ContentLength)
		}

		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatalf("ReadAll(Body) error = %v", err)
		}
		if string(body) != "hello video" {
			t.Fatalf("body = %q, want hello video", string(body))
		}
	})

	t.Run("empty file_id fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		_, err = client.File.Download(context.Background(), " ")
		if err == nil {
			t.Fatal("Download() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "file_id is empty") {
			t.Fatalf("Download() error = %v, want file_id validation error", err)
		}
	})

	t.Run("http error reads body and returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"missing"}`))
		}))
		defer srv.Close()

		client := newFileTestClient(t, srv)
		_, err := client.File.Download(context.Background(), "missing")
		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("Download() error type = %T, want *protocol.APIError", err)
		}
		if apiErr.HTTPStatus != http.StatusNotFound {
			t.Fatalf("apiErr.HTTPStatus = %d, want %d", apiErr.HTTPStatus, http.StatusNotFound)
		}
	})

	t.Run("json base_resp error is not exposed as file body", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":2013,"status_msg":"invalid file"}}`))
		}))
		defer srv.Close()

		client := newFileTestClient(t, srv)
		_, err := client.File.Download(context.Background(), "bad-file")
		assertAPIStatus(t, err, 2013, "invalid file")
	})
}

func TestFileDelete(t *testing.T) {
	t.Parallel()

	t.Run("success deletes file", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != defaultFileDeletePath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultFileDeletePath)
			}

			var payload struct {
				FileID  int64  `json:"file_id"`
				Purpose string `json:"purpose"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.FileID != 12345 || payload.Purpose != "voice_clone" {
				t.Fatalf("payload = %+v, want file_id=12345 purpose=voice_clone", payload)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"file_id":12345,"base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newFileTestClient(t, srv)
		response, err := client.File.Delete(context.Background(), FileDeleteRequest{
			FileID:  " 12345 ",
			Purpose: " voice_clone ",
		})
		if err != nil {
			t.Fatalf("Delete() error = %v, want nil", err)
		}
		if response.FileID != "12345" {
			t.Fatalf("response.FileID = %q, want 12345", response.FileID)
		}
	})

	t.Run("empty file_id fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		_, err = client.File.Delete(context.Background(), FileDeleteRequest{Purpose: "voice_clone"})
		if err == nil {
			t.Fatal("Delete() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "file_id is empty") {
			t.Fatalf("Delete() error = %v, want file_id validation error", err)
		}
	})

	t.Run("empty purpose fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		_, err = client.File.Delete(context.Background(), FileDeleteRequest{FileID: "12345"})
		if err == nil {
			t.Fatal("Delete() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "purpose is empty") {
			t.Fatalf("Delete() error = %v, want purpose validation error", err)
		}
	})

	t.Run("base_resp non-zero returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":2013,"status_msg":"invalid file"}}`))
		}))
		defer srv.Close()

		client := newFileTestClient(t, srv)
		_, err := client.File.Delete(context.Background(), FileDeleteRequest{
			FileID:  "12345",
			Purpose: "voice_clone",
		})
		assertAPIStatus(t, err, 2013, "invalid file")
	})
}

func newFileTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()

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

	return client
}

func assertAPIStatus(t *testing.T, err error, statusCode int, statusMsg string) {
	t.Helper()

	if err == nil {
		t.Fatal("error = nil, want non-nil")
	}

	var apiErr *protocol.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *protocol.APIError", err)
	}
	if apiErr.StatusCode != statusCode || apiErr.StatusMsg != statusMsg {
		t.Fatalf("apiErr = %+v, want status_code=%d status_msg=%s", apiErr, statusCode, statusMsg)
	}
}
