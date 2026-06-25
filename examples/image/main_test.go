package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseOptions(t *testing.T) {
	t.Parallel()

	t.Run("help is accepted", func(t *testing.T) {
		t.Parallel()

		var stderr bytes.Buffer
		_, err := parseOptions([]string{"-h"}, &stderr)
		if err == nil {
			t.Fatal("parseOptions() error = nil, want help error")
		}
		if !strings.Contains(stderr.String(), "Usage: go run ./examples/image") {
			t.Fatalf("help output = %q, want usage", stderr.String())
		}
	})

	t.Run("prompt is required", func(t *testing.T) {
		t.Parallel()

		_, err := parseOptions([]string{"-prompt", " ", "-model", "image-01"}, &bytes.Buffer{})
		if err == nil {
			t.Fatal("parseOptions() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "prompt is required") {
			t.Fatalf("parseOptions() error = %v, want prompt validation", err)
		}
	})

	t.Run("width and height must be paired", func(t *testing.T) {
		t.Parallel()

		_, err := parseOptions([]string{"-prompt", "desk robot", "-width", "1024"}, &bytes.Buffer{})
		if err == nil {
			t.Fatal("parseOptions() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "width and height") {
			t.Fatalf("parseOptions() error = %v, want dimension pair validation", err)
		}
	})
}

func TestRunGeneratesImageURLs(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/image_generation" {
			t.Fatalf("path = %s, want /v1/image_generation", r.URL.Path)
		}

		var payload struct {
			Model          string `json:"model"`
			Prompt         string `json:"prompt"`
			AspectRatio    string `json:"aspect_ratio"`
			ResponseFormat string `json:"response_format"`
			N              int    `json:"n"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if payload.Model != "image-01" || payload.Prompt != "desk robot" || payload.AspectRatio != "1:1" || payload.ResponseFormat != "url" || payload.N != 1 {
			t.Fatalf("payload = %+v, want expected image request", payload)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"img_example_123","data":{"image_urls":["https://example.com/image.png"]},"metadata":{"success_count":1,"failed_count":0},"base_resp":{"status_code":0,"status_msg":"success"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	err := run(options{
		apiKey:         "test-key",
		baseURL:        srv.URL,
		model:          "image-01",
		prompt:         "desk robot",
		aspectRatio:    "1:1",
		responseFormat: "url",
		n:              1,
		timeout:        30 * time.Second,
	}, &stdout)
	if err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}

	output := stdout.String()
	for _, want := range []string{
		"id=img_example_123",
		"success_count=1 failed_count=0",
		"image_url[0]=https://example.com/image.png",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
}

func TestRunSavesBase64Images(t *testing.T) {
	t.Parallel()

	encodedImage := base64.StdEncoding.EncodeToString([]byte("fake png data"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/image_generation" {
			t.Fatalf("path = %s, want /v1/image_generation", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"img_base64_123","data":{"image_base64":[` + strconvQuote(encodedImage) + `]},"metadata":{"success_count":1,"failed_count":0},"base_resp":{"status_code":0,"status_msg":"success"}}`))
	}))
	defer srv.Close()

	outputDir := t.TempDir()
	var stdout bytes.Buffer
	err := run(options{
		apiKey:         "test-key",
		baseURL:        srv.URL,
		model:          "image-01",
		prompt:         "desk robot",
		aspectRatio:    "1:1",
		responseFormat: "base64",
		n:              1,
		outputDir:      outputDir,
		timeout:        30 * time.Second,
	}, &stdout)
	if err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}

	outputPath := filepath.Join(outputDir, "image-01.png")
	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v, want nil", err)
	}
	if string(written) != "fake png data" {
		t.Fatalf("output file = %q, want fake png data", string(written))
	}
	if !strings.Contains(stdout.String(), "saved="+outputPath) {
		t.Fatalf("output = %q, want saved path", stdout.String())
	}
}

func TestRunRequiresAPIKey(t *testing.T) {
	t.Parallel()

	err := run(options{
		baseURL:        "https://api.minimax.io",
		model:          "image-01",
		prompt:         "desk robot",
		aspectRatio:    "1:1",
		responseFormat: "url",
		n:              1,
		timeout:        30 * time.Second,
	}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("run() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "missing API key") {
		t.Fatalf("run() error = %v, want missing API key", err)
	}
}

func strconvQuote(value string) string {
	payload, _ := json.Marshal(value)
	return string(payload)
}
