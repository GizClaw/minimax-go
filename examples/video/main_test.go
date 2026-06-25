package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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

	t.Run("image-to-video mode does not require prompt", func(t *testing.T) {
		t.Parallel()

		opts, err := parseOptions([]string{
			"-first-frame-image", " https://example.com/frame.png ",
			"-prompt", " ",
		}, &bytes.Buffer{})
		if err != nil {
			t.Fatalf("parseOptions() error = %v, want nil", err)
		}
		if opts.firstFrameImage != "https://example.com/frame.png" {
			t.Fatalf("firstFrameImage = %q, want trimmed image URL", opts.firstFrameImage)
		}
		if opts.prompt != "" {
			t.Fatalf("prompt = %q, want empty prompt", opts.prompt)
		}
	})

	t.Run("first-last-frame mode does not require prompt", func(t *testing.T) {
		t.Parallel()

		opts, err := parseOptions([]string{
			"-first-frame-image", " https://example.com/start.png ",
			"-last-frame-image", " https://example.com/end.png ",
			"-prompt", " ",
		}, &bytes.Buffer{})
		if err != nil {
			t.Fatalf("parseOptions() error = %v, want nil", err)
		}
		if opts.firstFrameImage != "https://example.com/start.png" {
			t.Fatalf("firstFrameImage = %q, want trimmed start image URL", opts.firstFrameImage)
		}
		if opts.lastFrameImage != "https://example.com/end.png" {
			t.Fatalf("lastFrameImage = %q, want trimmed end image URL", opts.lastFrameImage)
		}
		if opts.prompt != "" {
			t.Fatalf("prompt = %q, want empty prompt", opts.prompt)
		}
	})

	t.Run("first-last-frame mode does not require first frame", func(t *testing.T) {
		t.Parallel()

		opts, err := parseOptions([]string{
			"-last-frame-image", " https://example.com/end.png ",
			"-prompt", " ",
		}, &bytes.Buffer{})
		if err != nil {
			t.Fatalf("parseOptions() error = %v, want nil", err)
		}
		if opts.firstFrameImage != "" {
			t.Fatalf("firstFrameImage = %q, want empty", opts.firstFrameImage)
		}
		if opts.lastFrameImage != "https://example.com/end.png" {
			t.Fatalf("lastFrameImage = %q, want trimmed end image URL", opts.lastFrameImage)
		}
		if opts.prompt != "" {
			t.Fatalf("prompt = %q, want empty prompt", opts.prompt)
		}
	})
}

func TestRunSubmitsWaitsRetrievesAndDownloadsVideo(t *testing.T) {
	t.Parallel()

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

			_, _ = w.Write([]byte(fmt.Sprintf(`{"file":{"file_id":"file_video_123","download_url":%q},"base_resp":{"status_code":0,"status_msg":"success"}}`, srv.URL+"/video.mp4")))
		case "/v1/files/retrieve_content":
			if r.Method != http.MethodGet {
				t.Fatalf("retrieve_content method = %s, want GET", r.Method)
			}
			if got := r.URL.Query().Get("file_id"); got != "file_video_123" {
				t.Fatalf("retrieve_content file_id query = %q, want file_video_123", got)
			}

			_, _ = w.Write([]byte(`{"base_resp":{"status_code":2013,"status_msg":"invalid params, invalid file purpose"}}`))
		case "/video.mp4":
			if r.Method != http.MethodGet {
				t.Fatalf("download method = %s, want GET", r.Method)
			}
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("fake mp4 data"))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	outputPath := filepath.Join(t.TempDir(), "video.mp4")
	err := run(options{
		apiKey:          "test-key",
		baseURL:         srv.URL,
		model:           "MiniMax-Hailuo-2.3",
		prompt:          "desk robot",
		duration:        6,
		resolution:      "768P",
		promptOptimizer: true,
		wait:            true,
		output:          outputPath,
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
		"download_url=" + srv.URL + "/video.mp4",
		"saved=" + outputPath,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}

	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v, want nil", err)
	}
	if string(written) != "fake mp4 data" {
		t.Fatalf("output file = %q, want fake mp4 data", string(written))
	}
}

func TestRunSubmitsImageToVideo(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/v1/video_generation":
			if r.Method != http.MethodPost {
				t.Fatalf("submit method = %s, want POST", r.Method)
			}

			var payload struct {
				Model           string `json:"model"`
				FirstFrameImage string `json:"first_frame_image"`
				Prompt          string `json:"prompt,omitempty"`
				Duration        int    `json:"duration"`
				Resolution      string `json:"resolution"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.Model != "MiniMax-Hailuo-2.3" || payload.FirstFrameImage != "https://example.com/frame.png" || payload.Prompt != "camera pushes in" || payload.Duration != 6 || payload.Resolution != "1080P" {
				t.Fatalf("payload = %+v, want expected image-to-video submit payload", payload)
			}

			_, _ = w.Write([]byte(`{"task_id":"task_i2v_123","base_resp":{"status_code":0,"status_msg":"success"}}`))
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
		firstFrameImage: "https://example.com/frame.png",
		prompt:          "camera pushes in",
		duration:        6,
		resolution:      "1080P",
		promptOptimizer: true,
		timeout:         30 * time.Second,
		pollInterval:    time.Millisecond,
	}, &stdout)
	if err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "submitted task_id=task_i2v_123") {
		t.Fatalf("output = %q, want submitted task", output)
	}
}

func TestRunSubmitsImageToVideoWithoutPrompt(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path != "/v1/video_generation" {
			t.Fatalf("path = %s, want /v1/video_generation", r.URL.Path)
		}

		var payload map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if _, ok := payload["first_frame_image"]; !ok {
			t.Fatalf("payload = %+v, want first_frame_image", payload)
		}
		if _, ok := payload["prompt"]; ok {
			t.Fatalf("payload = %+v, want omitted empty prompt", payload)
		}

		_, _ = w.Write([]byte(`{"task_id":"task_i2v_no_prompt","base_resp":{"status_code":0,"status_msg":"success"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	err := run(options{
		apiKey:          "test-key",
		baseURL:         srv.URL,
		model:           "MiniMax-Hailuo-2.3",
		firstFrameImage: "https://example.com/frame.png",
		duration:        6,
		resolution:      "1080P",
		timeout:         30 * time.Second,
		pollInterval:    time.Millisecond,
	}, &stdout)
	if err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "submitted task_id=task_i2v_no_prompt") {
		t.Fatalf("output = %q, want submitted task", output)
	}
}

func TestRunSubmitsFirstLastFrameVideo(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/v1/video_generation":
			if r.Method != http.MethodPost {
				t.Fatalf("submit method = %s, want POST", r.Method)
			}

			var payload struct {
				Model           string `json:"model"`
				LastFrameImage  string `json:"last_frame_image"`
				FirstFrameImage string `json:"first_frame_image,omitempty"`
				Prompt          string `json:"prompt,omitempty"`
				Duration        int    `json:"duration"`
				Resolution      string `json:"resolution"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.Model != "MiniMax-Hailuo-02" || payload.LastFrameImage != "https://example.com/end.png" || payload.FirstFrameImage != "https://example.com/start.png" || payload.Prompt != "camera pulls back" || payload.Duration != 6 || payload.Resolution != "1080P" {
				t.Fatalf("payload = %+v, want expected first-last-frame submit payload", payload)
			}

			_, _ = w.Write([]byte(`{"task_id":"task_fl2v_123","base_resp":{"status_code":0,"status_msg":"success"}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	err := run(options{
		apiKey:          "test-key",
		baseURL:         srv.URL,
		model:           "MiniMax-Hailuo-02",
		lastFrameImage:  "https://example.com/end.png",
		firstFrameImage: "https://example.com/start.png",
		prompt:          "camera pulls back",
		duration:        6,
		resolution:      "1080P",
		promptOptimizer: true,
		timeout:         30 * time.Second,
		pollInterval:    time.Millisecond,
	}, &stdout)
	if err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "submitted task_id=task_fl2v_123") {
		t.Fatalf("output = %q, want submitted task", output)
	}
}

func TestRunSubmitsFirstLastFrameVideoWithoutPromptOrFirstFrame(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path != "/v1/video_generation" {
			t.Fatalf("path = %s, want /v1/video_generation", r.URL.Path)
		}

		var payload map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if _, ok := payload["last_frame_image"]; !ok {
			t.Fatalf("payload = %+v, want last_frame_image", payload)
		}
		if _, ok := payload["first_frame_image"]; ok {
			t.Fatalf("payload = %+v, want omitted empty first_frame_image", payload)
		}
		if _, ok := payload["prompt"]; ok {
			t.Fatalf("payload = %+v, want omitted empty prompt", payload)
		}

		_, _ = w.Write([]byte(`{"task_id":"task_fl2v_no_prompt","base_resp":{"status_code":0,"status_msg":"success"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	err := run(options{
		apiKey:         "test-key",
		baseURL:        srv.URL,
		model:          "MiniMax-Hailuo-02",
		lastFrameImage: "https://example.com/end.png",
		duration:       6,
		resolution:     "1080P",
		timeout:        30 * time.Second,
		pollInterval:   time.Millisecond,
	}, &stdout)
	if err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "submitted task_id=task_fl2v_no_prompt") {
		t.Fatalf("output = %q, want submitted task", output)
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
