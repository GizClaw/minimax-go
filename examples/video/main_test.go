package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		if !strings.Contains(stderr.String(), "Usage: go run ./examples/video") {
			t.Fatalf("help output = %q, want usage", stderr.String())
		}
	})

	t.Run("submit mode requires prompt", func(t *testing.T) {
		t.Parallel()

		_, err := parseOptions([]string{"-prompt", " ", "-model", "MiniMax-Hailuo-2.3"}, &bytes.Buffer{})
		if err == nil {
			t.Fatal("parseOptions() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "requires prompt") {
			t.Fatalf("parseOptions() error = %v, want prompt validation", err)
		}
	})

	t.Run("task mode does not require prompt", func(t *testing.T) {
		t.Parallel()

		opts, err := parseOptions([]string{"-task-id", " task_123 ", "-prompt", " "}, &bytes.Buffer{})
		if err != nil {
			t.Fatalf("parseOptions() error = %v, want nil", err)
		}
		if opts.taskID != "task_123" {
			t.Fatalf("taskID = %q, want task_123", opts.taskID)
		}
	})
}

func TestRunSubmitsWaitsAndRetrievesVideo(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/v1/video_generation":
			if r.Method != http.MethodPost {
				t.Fatalf("submit method = %s, want POST", r.Method)
			}

			var payload struct {
				Model      string `json:"model"`
				Prompt     string `json:"prompt"`
				Duration   int    `json:"duration"`
				Resolution string `json:"resolution"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.Model != "MiniMax-Hailuo-2.3" || payload.Prompt != "desk robot" || payload.Duration != 6 || payload.Resolution != "768P" {
				t.Fatalf("payload = %+v, want expected submit payload", payload)
			}

			_, _ = w.Write([]byte(`{"task_id":"task_video_123","base_resp":{"status_code":0,"status_msg":"success"}}`))
		case "/v1/query/video_generation":
			if r.Method != http.MethodGet {
				t.Fatalf("query method = %s, want GET", r.Method)
			}
			if got := r.URL.Query().Get("task_id"); got != "task_video_123" {
				t.Fatalf("task_id query = %q, want task_video_123", got)
			}

			_, _ = w.Write([]byte(`{"task_id":"task_video_123","status":"Success","file_id":"file_video_123","video_width":1280,"video_height":720,"base_resp":{"status_code":0,"status_msg":"success"}}`))
		case "/v1/files/retrieve":
			if r.Method != http.MethodGet {
				t.Fatalf("retrieve method = %s, want GET", r.Method)
			}
			if got := r.URL.Query().Get("file_id"); got != "file_video_123" {
				t.Fatalf("file_id query = %q, want file_video_123", got)
			}

			_, _ = w.Write([]byte(`{"file":{"file_id":"file_video_123","download_url":"https://cdn.example.com/video.mp4"},"base_resp":{"status_code":0,"status_msg":"success"}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	err := run(options{
		apiKey:          "test-key",
		baseURL:         srv.URL,
		model:           "MiniMax-Hailuo-2.3",
		prompt:          "desk robot",
		duration:        6,
		resolution:      "768P",
		promptOptimizer: true,
		wait:            true,
		timeout:         30 * time.Second,
		pollInterval:    time.Millisecond,
	}, &stdout)
	if err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}

	output := stdout.String()
	for _, want := range []string{
		"submitted task_id=task_video_123",
		"task_id=task_video_123 status=success raw_status=Success file_id=file_video_123",
		"file_id=file_video_123",
		"download_url=https://cdn.example.com/video.mp4",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
}

func TestRunRequiresAPIKey(t *testing.T) {
	t.Parallel()

	err := run(options{
		baseURL:      "https://api.minimax.io",
		model:        "MiniMax-Hailuo-2.3",
		prompt:       "desk robot",
		timeout:      30 * time.Second,
		pollInterval: time.Second,
	}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("run() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "missing API key") {
		t.Fatalf("run() error = %v, want missing API key", err)
	}
}
