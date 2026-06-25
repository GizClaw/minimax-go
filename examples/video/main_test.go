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

	t.Run("subject-reference mode does not require prompt", func(t *testing.T) {
		t.Parallel()

		opts, err := parseOptions([]string{
			"-model", " S2V-01 ",
			"-subject-reference", " character=https://example.com/person.png ",
			"-prompt", " ",
		}, &bytes.Buffer{})
		if err != nil {
			t.Fatalf("parseOptions() error = %v, want nil", err)
		}
		if opts.model != "S2V-01" {
			t.Fatalf("model = %q, want S2V-01", opts.model)
		}
		if opts.prompt != "" {
			t.Fatalf("prompt = %q, want empty prompt", opts.prompt)
		}
		references := opts.subjectRefs.VideoSubjectReferences()
		if len(references) != 1 {
			t.Fatalf("len(references) = %d, want 1", len(references))
		}
		if references[0].Type != "character" || len(references[0].Image) != 1 || references[0].Image[0] != "https://example.com/person.png" {
			t.Fatalf("references = %+v, want character subject reference", references)
		}
	})

	t.Run("invalid subject-reference fails", func(t *testing.T) {
		t.Parallel()

		_, err := parseOptions([]string{
			"-subject-reference", " character ",
		}, &bytes.Buffer{})
		if err == nil {
			t.Fatal("parseOptions() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "subject-reference must be formatted as type=image_url") {
			t.Fatalf("parseOptions() error = %v, want subject-reference validation", err)
		}
	})
}

func TestEnvironmentDefaultHelpers(t *testing.T) {
	t.Run("string env uses explicit value and default fallback", func(t *testing.T) {
		t.Setenv("MINIMAX_TEST_STRING", " custom ")
		if got := envOrDefault("MINIMAX_TEST_STRING", "fallback"); got != " custom " {
			t.Fatalf("envOrDefault() = %q, want explicit value", got)
		}
		if got := envOrDefault("MINIMAX_TEST_STRING_MISSING", "fallback"); got != "fallback" {
			t.Fatalf("envOrDefault() = %q, want fallback", got)
		}
	})

	t.Run("duration env parses valid values and falls back on invalid input", func(t *testing.T) {
		t.Setenv("MINIMAX_TEST_DURATION", "250ms")
		if got := envDurationOrDefault("MINIMAX_TEST_DURATION", time.Second); got != 250*time.Millisecond {
			t.Fatalf("envDurationOrDefault() = %s, want 250ms", got)
		}
		t.Setenv("MINIMAX_TEST_DURATION", "-1s")
		if got := envDurationOrDefault("MINIMAX_TEST_DURATION", time.Second); got != time.Second {
			t.Fatalf("envDurationOrDefault() = %s, want fallback", got)
		}
	})

	t.Run("int env parses valid values and falls back on invalid input", func(t *testing.T) {
		t.Setenv("MINIMAX_TEST_INT", "42")
		if got := envIntOrDefault("MINIMAX_TEST_INT", 7); got != 42 {
			t.Fatalf("envIntOrDefault() = %d, want 42", got)
		}
		t.Setenv("MINIMAX_TEST_INT", "nope")
		if got := envIntOrDefault("MINIMAX_TEST_INT", 7); got != 7 {
			t.Fatalf("envIntOrDefault() = %d, want fallback", got)
		}
	})

	t.Run("bool env parses true false and falls back on invalid input", func(t *testing.T) {
		t.Setenv("MINIMAX_TEST_BOOL", "yes")
		if got := envBoolOrDefault("MINIMAX_TEST_BOOL", false); !got {
			t.Fatal("envBoolOrDefault() = false, want true")
		}
		t.Setenv("MINIMAX_TEST_BOOL", "off")
		if got := envBoolOrDefault("MINIMAX_TEST_BOOL", true); got {
			t.Fatal("envBoolOrDefault() = true, want false")
		}
		t.Setenv("MINIMAX_TEST_BOOL", "maybe")
		if got := envBoolOrDefault("MINIMAX_TEST_BOOL", true); !got {
			t.Fatal("envBoolOrDefault() = false, want fallback true")
		}
	})
}

func TestSubjectReferenceFlagsString(t *testing.T) {
	t.Parallel()

	var empty subjectReferenceFlags
	if got := empty.String(); got != "" {
		t.Fatalf("empty.String() = %q, want empty", got)
	}

	references := subjectReferenceFlags{
		{referenceType: "character", imageURL: "https://example.com/a.png"},
		{referenceType: "character", imageURL: "https://example.com/b.png"},
	}
	if got := references.String(); got != "character=https://example.com/a.png,character=https://example.com/b.png" {
		t.Fatalf("references.String() = %q, want joined references", got)
	}
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

func TestRunQueriesExistingTaskAsJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path != "/v1/query/video_generation" {
			t.Fatalf("path = %s, want /v1/query/video_generation", r.URL.Path)
		}
		if got := r.URL.Query().Get("task_id"); got != "task_existing" {
			t.Fatalf("task_id query = %q, want task_existing", got)
		}

		_, _ = w.Write([]byte(`{"task_id":"task_existing","status":"Processing","base_resp":{"status_code":0,"status_msg":"success"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	err := run(options{
		apiKey:       "test-key",
		baseURL:      srv.URL,
		taskID:       "task_existing",
		timeout:      30 * time.Second,
		pollInterval: time.Millisecond,
		asJSON:       true,
	}, &stdout)
	if err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "task_id=task_existing status=processing raw_status=Processing file_id=") {
		t.Fatalf("output = %q, want task status line", output)
	}
	if !strings.Contains(output, `"task_id": "task_existing"`) || !strings.Contains(output, `"status": "processing"`) {
		t.Fatalf("output = %q, want formatted JSON response", output)
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

func TestRunSubmitsSubjectReferenceVideo(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/v1/video_generation":
			if r.Method != http.MethodPost {
				t.Fatalf("submit method = %s, want POST", r.Method)
			}

			var payload struct {
				Model             string `json:"model"`
				SubjectReferences []struct {
					Type  string   `json:"type"`
					Image []string `json:"image"`
				} `json:"subject_reference"`
				Prompt          string `json:"prompt,omitempty"`
				PromptOptimizer bool   `json:"prompt_optimizer"`
				CallbackURL     string `json:"callback_url,omitempty"`
				AIGCWatermark   bool   `json:"aigc_watermark"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.Model != "S2V-01" || payload.Prompt != "smile and wave" || !payload.PromptOptimizer || payload.CallbackURL != "https://callback.example.com/video" || payload.AIGCWatermark {
				t.Fatalf("payload = %+v, want expected subject-reference submit payload", payload)
			}
			if len(payload.SubjectReferences) != 1 {
				t.Fatalf("len(payload.SubjectReferences) = %d, want 1", len(payload.SubjectReferences))
			}
			reference := payload.SubjectReferences[0]
			if reference.Type != "character" || len(reference.Image) != 1 || reference.Image[0] != "https://example.com/person.png" {
				t.Fatalf("reference = %+v, want expected subject reference", reference)
			}

			_, _ = w.Write([]byte(`{"task_id":"task_s2v_123","base_resp":{"status_code":0,"status_msg":"success"}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	err := run(options{
		apiKey:          "test-key",
		baseURL:         srv.URL,
		model:           "S2V-01",
		subjectRefs:     subjectReferenceFlags{{referenceType: "character", imageURL: "https://example.com/person.png"}},
		prompt:          "smile and wave",
		callbackURL:     "https://callback.example.com/video",
		promptOptimizer: true,
		timeout:         30 * time.Second,
		pollInterval:    time.Millisecond,
	}, &stdout)
	if err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "submitted task_id=task_s2v_123") {
		t.Fatalf("output = %q, want submitted task", output)
	}
}

func TestRunSubmitsSubjectReferenceVideoWithoutPrompt(t *testing.T) {
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
		if _, ok := payload["subject_reference"]; !ok {
			t.Fatalf("payload = %+v, want subject_reference", payload)
		}
		if _, ok := payload["prompt"]; ok {
			t.Fatalf("payload = %+v, want omitted empty prompt", payload)
		}

		_, _ = w.Write([]byte(`{"task_id":"task_s2v_no_prompt","base_resp":{"status_code":0,"status_msg":"success"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	err := run(options{
		apiKey:       "test-key",
		baseURL:      srv.URL,
		model:        "S2V-01",
		subjectRefs:  subjectReferenceFlags{{referenceType: "character", imageURL: "https://example.com/person.png"}},
		timeout:      30 * time.Second,
		pollInterval: time.Millisecond,
	}, &stdout)
	if err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "submitted task_id=task_s2v_no_prompt") {
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
