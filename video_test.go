package minimax

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GizClaw/minimax-go/internal/protocol"
	"github.com/GizClaw/minimax-go/internal/transport"
)

func TestVideoCreateTextToVideo(t *testing.T) {
	t.Parallel()

	t.Run("success creates text-to-video task", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != defaultVideoGenerationPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultVideoGenerationPath)
			}

			var payload VideoTextToVideoRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.Model != "MiniMax-Hailuo-2.3" {
				t.Fatalf("payload.Model = %q, want MiniMax-Hailuo-2.3", payload.Model)
			}
			if payload.Prompt != "A man picks up a book [Pedestal up]" {
				t.Fatalf("payload.Prompt = %q, want trimmed prompt", payload.Prompt)
			}
			if payload.PromptOptimizer == nil || *payload.PromptOptimizer {
				t.Fatalf("payload.PromptOptimizer = %v, want explicit false", payload.PromptOptimizer)
			}
			if payload.FastPretreatment == nil || !*payload.FastPretreatment {
				t.Fatalf("payload.FastPretreatment = %v, want explicit true", payload.FastPretreatment)
			}
			if payload.Duration == nil || *payload.Duration != 6 {
				t.Fatalf("payload.Duration = %v, want 6", payload.Duration)
			}
			if payload.Resolution != "1080P" || payload.CallbackURL != "https://callback.example.com/video" {
				t.Fatalf("payload resolution/callback = %q/%q", payload.Resolution, payload.CallbackURL)
			}
			if payload.AIGCWatermark == nil || !*payload.AIGCWatermark {
				t.Fatalf("payload.AIGCWatermark = %v, want explicit true", payload.AIGCWatermark)
			}

			w.Header().Set("X-Trace-ID", "trace-video-create")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"task_id":"106916112212032","extra":"kept","base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		response, err := client.Video.CreateTextToVideo(context.Background(), VideoTextToVideoRequest{
			Model:            " MiniMax-Hailuo-2.3 ",
			Prompt:           " A man picks up a book [Pedestal up] ",
			PromptOptimizer:  videoBoolPtr(false),
			FastPretreatment: videoBoolPtr(true),
			Duration:         videoIntPtr(6),
			Resolution:       " 1080P ",
			CallbackURL:      " https://callback.example.com/video ",
			AIGCWatermark:    videoBoolPtr(true),
		})
		if err != nil {
			t.Fatalf("CreateTextToVideo() error = %v, want nil", err)
		}
		if response.TaskID != "106916112212032" {
			t.Fatalf("response.TaskID = %q, want 106916112212032", response.TaskID)
		}
		if response.ResponseMeta.TraceID != "trace-video-create" {
			t.Fatalf("TraceID = %q, want trace-video-create", response.ResponseMeta.TraceID)
		}
		if _, ok := response.Raw["extra"]; !ok {
			t.Fatalf("response.Raw missing extra field: %+v", response.Raw)
		}
	})

	t.Run("nested data task_id is accepted", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"task_id":106916112212032},"base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		response, err := client.Video.CreateTextToVideo(context.Background(), VideoTextToVideoRequest{
			Model:  "MiniMax-Hailuo-2.3",
			Prompt: "A quiet city street",
		})
		if err != nil {
			t.Fatalf("CreateTextToVideo() error = %v, want nil", err)
		}
		if response.TaskID != "106916112212032" {
			t.Fatalf("response.TaskID = %q, want 106916112212032", response.TaskID)
		}
	})

	t.Run("empty model fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Video.CreateTextToVideo(context.Background(), VideoTextToVideoRequest{Prompt: "hello"})
		if err == nil {
			t.Fatal("CreateTextToVideo() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "model is empty") {
			t.Fatalf("CreateTextToVideo() error = %v, want model validation error", err)
		}
	})

	t.Run("empty prompt fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Video.CreateTextToVideo(context.Background(), VideoTextToVideoRequest{Model: "MiniMax-Hailuo-2.3"})
		if err == nil {
			t.Fatal("CreateTextToVideo() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "prompt is empty") {
			t.Fatalf("CreateTextToVideo() error = %v, want prompt validation error", err)
		}
	})

	t.Run("base_resp non-zero returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":1026,"status_msg":"sensitive prompt"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		_, err := client.Video.CreateTextToVideo(context.Background(), VideoTextToVideoRequest{
			Model:  "MiniMax-Hailuo-2.3",
			Prompt: "hello",
		})
		assertAPIStatus(t, err, 1026, "sensitive prompt")
	})

	t.Run("context canceled is preserved", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = client.Video.CreateTextToVideo(ctx, VideoTextToVideoRequest{
			Model:  "MiniMax-Hailuo-2.3",
			Prompt: "hello",
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("CreateTextToVideo() error = %v, want context canceled", err)
		}
	})
}

func TestVideoCreateImageToVideo(t *testing.T) {
	t.Parallel()

	t.Run("success creates image-to-video task", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != defaultVideoGenerationPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultVideoGenerationPath)
			}

			var payload VideoImageToVideoRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.Model != "MiniMax-Hailuo-2.3" {
				t.Fatalf("payload.Model = %q, want MiniMax-Hailuo-2.3", payload.Model)
			}
			if payload.FirstFrameImage != "https://example.com/frame.png" {
				t.Fatalf("payload.FirstFrameImage = %q, want trimmed image URL", payload.FirstFrameImage)
			}
			if payload.Prompt != "A mouse runs toward the camera" {
				t.Fatalf("payload.Prompt = %q, want trimmed prompt", payload.Prompt)
			}
			if payload.PromptOptimizer == nil || *payload.PromptOptimizer {
				t.Fatalf("payload.PromptOptimizer = %v, want explicit false", payload.PromptOptimizer)
			}
			if payload.FastPretreatment == nil || !*payload.FastPretreatment {
				t.Fatalf("payload.FastPretreatment = %v, want explicit true", payload.FastPretreatment)
			}
			if payload.Duration == nil || *payload.Duration != 6 {
				t.Fatalf("payload.Duration = %v, want 6", payload.Duration)
			}
			if payload.Resolution != "1080P" || payload.CallbackURL != "https://callback.example.com/video" {
				t.Fatalf("payload resolution/callback = %q/%q", payload.Resolution, payload.CallbackURL)
			}
			if payload.AIGCWatermark == nil || !*payload.AIGCWatermark {
				t.Fatalf("payload.AIGCWatermark = %v, want explicit true", payload.AIGCWatermark)
			}

			w.Header().Set("X-Trace-ID", "trace-video-i2v")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"task_id":"106916112212033","extra":"kept","base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		response, err := client.Video.CreateImageToVideo(context.Background(), VideoImageToVideoRequest{
			Model:            " MiniMax-Hailuo-2.3 ",
			FirstFrameImage:  " https://example.com/frame.png ",
			Prompt:           " A mouse runs toward the camera ",
			PromptOptimizer:  videoBoolPtr(false),
			FastPretreatment: videoBoolPtr(true),
			Duration:         videoIntPtr(6),
			Resolution:       " 1080P ",
			CallbackURL:      " https://callback.example.com/video ",
			AIGCWatermark:    videoBoolPtr(true),
		})
		if err != nil {
			t.Fatalf("CreateImageToVideo() error = %v, want nil", err)
		}
		if response.TaskID != "106916112212033" {
			t.Fatalf("response.TaskID = %q, want 106916112212033", response.TaskID)
		}
		if response.ResponseMeta.TraceID != "trace-video-i2v" {
			t.Fatalf("TraceID = %q, want trace-video-i2v", response.ResponseMeta.TraceID)
		}
		if _, ok := response.Raw["extra"]; !ok {
			t.Fatalf("response.Raw missing extra field: %+v", response.Raw)
		}
	})

	t.Run("nested data task_id is accepted", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"task_id":106916112212033},"base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		response, err := client.Video.CreateImageToVideo(context.Background(), VideoImageToVideoRequest{
			Model:           "MiniMax-Hailuo-2.3",
			FirstFrameImage: "https://example.com/frame.png",
		})
		if err != nil {
			t.Fatalf("CreateImageToVideo() error = %v, want nil", err)
		}
		if response.TaskID != "106916112212033" {
			t.Fatalf("response.TaskID = %q, want 106916112212033", response.TaskID)
		}
	})

	t.Run("empty model fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Video.CreateImageToVideo(context.Background(), VideoImageToVideoRequest{
			FirstFrameImage: "https://example.com/frame.png",
		})
		if err == nil {
			t.Fatal("CreateImageToVideo() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "model is empty") {
			t.Fatalf("CreateImageToVideo() error = %v, want model validation error", err)
		}
	})

	t.Run("empty first frame image fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Video.CreateImageToVideo(context.Background(), VideoImageToVideoRequest{
			Model:           "MiniMax-Hailuo-2.3",
			FirstFrameImage: " ",
		})
		if err == nil {
			t.Fatal("CreateImageToVideo() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "first_frame_image is empty") {
			t.Fatalf("CreateImageToVideo() error = %v, want first_frame_image validation error", err)
		}
	})

	t.Run("http 5xx returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"temporary unavailable"}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		_, err := client.Video.CreateImageToVideo(context.Background(), VideoImageToVideoRequest{
			Model:           "MiniMax-Hailuo-2.3",
			FirstFrameImage: "https://example.com/frame.png",
		})
		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("CreateImageToVideo() error type = %T, want *protocol.APIError", err)
		}
		if apiErr.HTTPStatus != http.StatusServiceUnavailable {
			t.Fatalf("apiErr.HTTPStatus = %d, want %d", apiErr.HTTPStatus, http.StatusServiceUnavailable)
		}
	})

	t.Run("base_resp non-zero returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":2013,"status_msg":"invalid image"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		_, err := client.Video.CreateImageToVideo(context.Background(), VideoImageToVideoRequest{
			Model:           "MiniMax-Hailuo-2.3",
			FirstFrameImage: "https://example.com/frame.png",
		})
		assertAPIStatus(t, err, 2013, "invalid image")
	})

	t.Run("missing task_id fails", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		_, err := client.Video.CreateImageToVideo(context.Background(), VideoImageToVideoRequest{
			Model:           "MiniMax-Hailuo-2.3",
			FirstFrameImage: "https://example.com/frame.png",
		})
		if err == nil || !strings.Contains(err.Error(), "response missing task_id") {
			t.Fatalf("CreateImageToVideo() error = %v, want missing task_id error", err)
		}
	})

	t.Run("context canceled is preserved", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = client.Video.CreateImageToVideo(ctx, VideoImageToVideoRequest{
			Model:           "MiniMax-Hailuo-2.3",
			FirstFrameImage: "https://example.com/frame.png",
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("CreateImageToVideo() error = %v, want context canceled", err)
		}
	})
}

func TestVideoCreateFirstLastFrameVideo(t *testing.T) {
	t.Parallel()

	t.Run("success creates first-last-frame task", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != defaultVideoGenerationPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultVideoGenerationPath)
			}

			var payload VideoFirstLastFrameRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.Model != "MiniMax-Hailuo-02" {
				t.Fatalf("payload.Model = %q, want MiniMax-Hailuo-02", payload.Model)
			}
			if payload.LastFrameImage != "https://example.com/end.png" {
				t.Fatalf("payload.LastFrameImage = %q, want trimmed last frame URL", payload.LastFrameImage)
			}
			if payload.FirstFrameImage != "https://example.com/start.png" {
				t.Fatalf("payload.FirstFrameImage = %q, want trimmed first frame URL", payload.FirstFrameImage)
			}
			if payload.Prompt != "A child grows up in a sunny garden" {
				t.Fatalf("payload.Prompt = %q, want trimmed prompt", payload.Prompt)
			}
			if payload.PromptOptimizer == nil || *payload.PromptOptimizer {
				t.Fatalf("payload.PromptOptimizer = %v, want explicit false", payload.PromptOptimizer)
			}
			if payload.Duration == nil || *payload.Duration != 6 {
				t.Fatalf("payload.Duration = %v, want 6", payload.Duration)
			}
			if payload.Resolution != "1080P" || payload.CallbackURL != "https://callback.example.com/video" {
				t.Fatalf("payload resolution/callback = %q/%q", payload.Resolution, payload.CallbackURL)
			}
			if payload.AIGCWatermark == nil || !*payload.AIGCWatermark {
				t.Fatalf("payload.AIGCWatermark = %v, want explicit true", payload.AIGCWatermark)
			}

			w.Header().Set("X-Trace-ID", "trace-video-fl2v")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"task_id":"106916112212034","extra":"kept","base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		response, err := client.Video.CreateFirstLastFrameVideo(context.Background(), VideoFirstLastFrameRequest{
			Model:           " MiniMax-Hailuo-02 ",
			LastFrameImage:  " https://example.com/end.png ",
			FirstFrameImage: " https://example.com/start.png ",
			Prompt:          " A child grows up in a sunny garden ",
			PromptOptimizer: videoBoolPtr(false),
			Duration:        videoIntPtr(6),
			Resolution:      " 1080P ",
			CallbackURL:     " https://callback.example.com/video ",
			AIGCWatermark:   videoBoolPtr(true),
		})
		if err != nil {
			t.Fatalf("CreateFirstLastFrameVideo() error = %v, want nil", err)
		}
		if response.TaskID != "106916112212034" {
			t.Fatalf("response.TaskID = %q, want 106916112212034", response.TaskID)
		}
		if response.ResponseMeta.TraceID != "trace-video-fl2v" {
			t.Fatalf("TraceID = %q, want trace-video-fl2v", response.ResponseMeta.TraceID)
		}
		if _, ok := response.Raw["extra"]; !ok {
			t.Fatalf("response.Raw missing extra field: %+v", response.Raw)
		}
	})

	t.Run("last frame without first frame is accepted", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"task_id":106916112212034},"base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		response, err := client.Video.CreateFirstLastFrameVideo(context.Background(), VideoFirstLastFrameRequest{
			Model:          "MiniMax-Hailuo-02",
			LastFrameImage: "https://example.com/end.png",
		})
		if err != nil {
			t.Fatalf("CreateFirstLastFrameVideo() error = %v, want nil", err)
		}
		if response.TaskID != "106916112212034" {
			t.Fatalf("response.TaskID = %q, want 106916112212034", response.TaskID)
		}
	})

	t.Run("empty model fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Video.CreateFirstLastFrameVideo(context.Background(), VideoFirstLastFrameRequest{
			LastFrameImage: "https://example.com/end.png",
		})
		if err == nil {
			t.Fatal("CreateFirstLastFrameVideo() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "model is empty") {
			t.Fatalf("CreateFirstLastFrameVideo() error = %v, want model validation error", err)
		}
	})

	t.Run("empty last frame image fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Video.CreateFirstLastFrameVideo(context.Background(), VideoFirstLastFrameRequest{
			Model:          "MiniMax-Hailuo-02",
			LastFrameImage: " ",
		})
		if err == nil {
			t.Fatal("CreateFirstLastFrameVideo() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "last_frame_image is empty") {
			t.Fatalf("CreateFirstLastFrameVideo() error = %v, want last_frame_image validation error", err)
		}
	})

	t.Run("base_resp non-zero returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":2013,"status_msg":"invalid last frame"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		_, err := client.Video.CreateFirstLastFrameVideo(context.Background(), VideoFirstLastFrameRequest{
			Model:          "MiniMax-Hailuo-02",
			LastFrameImage: "https://example.com/end.png",
		})
		assertAPIStatus(t, err, 2013, "invalid last frame")
	})

	t.Run("context canceled is preserved", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = client.Video.CreateFirstLastFrameVideo(ctx, VideoFirstLastFrameRequest{
			Model:          "MiniMax-Hailuo-02",
			LastFrameImage: "https://example.com/end.png",
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("CreateFirstLastFrameVideo() error = %v, want context canceled", err)
		}
	})
}

func TestVideoCreateSubjectReferenceVideo(t *testing.T) {
	t.Parallel()

	t.Run("success creates subject-reference task", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != defaultVideoGenerationPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultVideoGenerationPath)
			}

			var payload VideoSubjectReferenceRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.Model != "S2V-01" {
				t.Fatalf("payload.Model = %q, want S2V-01", payload.Model)
			}
			if payload.Prompt != "A girl runs toward the camera and smiles" {
				t.Fatalf("payload.Prompt = %q, want trimmed prompt", payload.Prompt)
			}
			if payload.PromptOptimizer == nil || *payload.PromptOptimizer {
				t.Fatalf("payload.PromptOptimizer = %v, want explicit false", payload.PromptOptimizer)
			}
			if payload.CallbackURL != "https://callback.example.com/video" {
				t.Fatalf("payload.CallbackURL = %q, want trimmed callback URL", payload.CallbackURL)
			}
			if payload.AIGCWatermark == nil || !*payload.AIGCWatermark {
				t.Fatalf("payload.AIGCWatermark = %v, want explicit true", payload.AIGCWatermark)
			}
			if len(payload.SubjectReferences) != 1 {
				t.Fatalf("len(payload.SubjectReferences) = %d, want 1", len(payload.SubjectReferences))
			}
			reference := payload.SubjectReferences[0]
			if reference.Type != "character" {
				t.Fatalf("reference.Type = %q, want character", reference.Type)
			}
			if len(reference.Image) != 1 || reference.Image[0] != "https://example.com/person.png" {
				t.Fatalf("reference.Image = %#v, want one trimmed image URL", reference.Image)
			}

			w.Header().Set("X-Trace-ID", "trace-video-s2v")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"task_id":"106916112212035","extra":"kept","base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		response, err := client.Video.CreateSubjectReferenceVideo(context.Background(), VideoSubjectReferenceRequest{
			Model: " S2V-01 ",
			SubjectReferences: []VideoSubjectReference{
				{Type: " character ", Image: []string{" https://example.com/person.png "}},
			},
			Prompt:          " A girl runs toward the camera and smiles ",
			PromptOptimizer: videoBoolPtr(false),
			CallbackURL:     " https://callback.example.com/video ",
			AIGCWatermark:   videoBoolPtr(true),
		})
		if err != nil {
			t.Fatalf("CreateSubjectReferenceVideo() error = %v, want nil", err)
		}
		if response.TaskID != "106916112212035" {
			t.Fatalf("response.TaskID = %q, want 106916112212035", response.TaskID)
		}
		if response.ResponseMeta.TraceID != "trace-video-s2v" {
			t.Fatalf("TraceID = %q, want trace-video-s2v", response.ResponseMeta.TraceID)
		}
		if _, ok := response.Raw["extra"]; !ok {
			t.Fatalf("response.Raw missing extra field: %+v", response.Raw)
		}
	})

	t.Run("subject reference without prompt is accepted", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"task_id":106916112212035},"base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		response, err := client.Video.CreateSubjectReferenceVideo(context.Background(), VideoSubjectReferenceRequest{
			Model: "S2V-01",
			SubjectReferences: []VideoSubjectReference{
				{Type: "character", Image: []string{"https://example.com/person.png"}},
			},
		})
		if err != nil {
			t.Fatalf("CreateSubjectReferenceVideo() error = %v, want nil", err)
		}
		if response.TaskID != "106916112212035" {
			t.Fatalf("response.TaskID = %q, want 106916112212035", response.TaskID)
		}
	})

	t.Run("empty model fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Video.CreateSubjectReferenceVideo(context.Background(), VideoSubjectReferenceRequest{
			SubjectReferences: []VideoSubjectReference{
				{Type: "character", Image: []string{"https://example.com/person.png"}},
			},
		})
		if err == nil {
			t.Fatal("CreateSubjectReferenceVideo() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "model is empty") {
			t.Fatalf("CreateSubjectReferenceVideo() error = %v, want model validation error", err)
		}
	})

	t.Run("empty subject reference fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Video.CreateSubjectReferenceVideo(context.Background(), VideoSubjectReferenceRequest{
			Model: "S2V-01",
		})
		if err == nil {
			t.Fatal("CreateSubjectReferenceVideo() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "subject_reference is empty") {
			t.Fatalf("CreateSubjectReferenceVideo() error = %v, want subject_reference validation error", err)
		}
	})

	t.Run("empty subject reference type fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Video.CreateSubjectReferenceVideo(context.Background(), VideoSubjectReferenceRequest{
			Model: "S2V-01",
			SubjectReferences: []VideoSubjectReference{
				{Type: " ", Image: []string{"https://example.com/person.png"}},
			},
		})
		if err == nil {
			t.Fatal("CreateSubjectReferenceVideo() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "subject_reference[0].type is empty") {
			t.Fatalf("CreateSubjectReferenceVideo() error = %v, want type validation error", err)
		}
	})

	t.Run("empty subject reference image list fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Video.CreateSubjectReferenceVideo(context.Background(), VideoSubjectReferenceRequest{
			Model: "S2V-01",
			SubjectReferences: []VideoSubjectReference{
				{Type: "character"},
			},
		})
		if err == nil {
			t.Fatal("CreateSubjectReferenceVideo() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "subject_reference[0].image is empty") {
			t.Fatalf("CreateSubjectReferenceVideo() error = %v, want image validation error", err)
		}
	})

	t.Run("empty subject reference image item fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Video.CreateSubjectReferenceVideo(context.Background(), VideoSubjectReferenceRequest{
			Model: "S2V-01",
			SubjectReferences: []VideoSubjectReference{
				{Type: "character", Image: []string{" "}},
			},
		})
		if err == nil {
			t.Fatal("CreateSubjectReferenceVideo() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "subject_reference[0].image[0] is empty") {
			t.Fatalf("CreateSubjectReferenceVideo() error = %v, want image item validation error", err)
		}
	})

	t.Run("base_resp non-zero returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":2013,"status_msg":"invalid subject reference"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		_, err := client.Video.CreateSubjectReferenceVideo(context.Background(), VideoSubjectReferenceRequest{
			Model: "S2V-01",
			SubjectReferences: []VideoSubjectReference{
				{Type: "character", Image: []string{"https://example.com/person.png"}},
			},
		})
		assertAPIStatus(t, err, 2013, "invalid subject reference")
	})

	t.Run("context canceled is preserved", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = client.Video.CreateSubjectReferenceVideo(ctx, VideoSubjectReferenceRequest{
			Model: "S2V-01",
			SubjectReferences: []VideoSubjectReference{
				{Type: "character", Image: []string{"https://example.com/person.png"}},
			},
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("CreateSubjectReferenceVideo() error = %v, want context canceled", err)
		}
	})
}

func TestVideoGetTask(t *testing.T) {
	t.Parallel()

	t.Run("success maps completed task", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Fatalf("method = %s, want GET", r.Method)
			}
			if r.URL.Path != defaultVideoQueryPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultVideoQueryPath)
			}
			if got := r.URL.Query().Get("task_id"); got != "176843862716480" {
				t.Fatalf("task_id query = %q, want 176843862716480", got)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"task_id":"176843862716480","status":"Success","file_id":"176844028768320","video_width":1920,"video_height":1080,"extra":"kept","base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		response, err := client.Video.GetTask(context.Background(), " 176843862716480 ")
		if err != nil {
			t.Fatalf("GetTask() error = %v, want nil", err)
		}
		if response.TaskID != "176843862716480" {
			t.Fatalf("TaskID = %q, want 176843862716480", response.TaskID)
		}
		if response.Status != VideoTaskStateSucceeded || response.RawStatus != "Success" {
			t.Fatalf("status = %q raw=%q, want success/Success", response.Status, response.RawStatus)
		}
		if response.FileID != "176844028768320" {
			t.Fatalf("FileID = %q, want 176844028768320", response.FileID)
		}
		if response.VideoWidth == nil || *response.VideoWidth != 1920 || response.VideoHeight == nil || *response.VideoHeight != 1080 {
			t.Fatalf("dimensions = %v x %v, want 1920 x 1080", response.VideoWidth, response.VideoHeight)
		}
		if _, ok := response.Raw["extra"]; !ok {
			t.Fatalf("response.Raw missing extra field: %+v", response.Raw)
		}
	})

	t.Run("processing state normalizes active statuses", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"task_id":"task_processing","status":"Queueing","base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		response, err := client.Video.GetTask(context.Background(), "task_processing")
		if err != nil {
			t.Fatalf("GetTask() error = %v, want nil", err)
		}
		if response.Status != VideoTaskStateProcessing || response.RawStatus != "Queueing" {
			t.Fatalf("status = %q raw=%q, want processing/Queueing", response.Status, response.RawStatus)
		}
	})

	t.Run("failed state maps failure details", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"task_id":"task_failed","status":"Fail","failure_code":1027,"failure_msg":"unsafe output","base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		response, err := client.Video.GetTask(context.Background(), "task_failed")
		if err != nil {
			t.Fatalf("GetTask() error = %v, want nil", err)
		}
		if response.Status != VideoTaskStateFailed || response.FailureCode != "1027" || response.FailureMsg != "unsafe output" {
			t.Fatalf("response = %+v, want failed task with failure details", response)
		}
	})

	t.Run("unknown status preserves raw status", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"task_id":"task_custom","status":"ThrottledButRetriable","base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		response, err := client.Video.GetTask(context.Background(), "task_custom")
		if err != nil {
			t.Fatalf("GetTask() error = %v, want nil", err)
		}
		if response.Status != "" || response.RawStatus != "ThrottledButRetriable" {
			t.Fatalf("status = %q raw=%q, want empty/ThrottledButRetriable", response.Status, response.RawStatus)
		}
	})

	t.Run("nested payload is accepted", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"task_id":176843862716480,"status":"Success","file_id":176844028768320,"video_width":1280,"video_height":720},"base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		response, err := client.Video.GetTask(context.Background(), "fallback-task")
		if err != nil {
			t.Fatalf("GetTask() error = %v, want nil", err)
		}
		if response.TaskID != "176843862716480" || response.FileID != "176844028768320" {
			t.Fatalf("response IDs = %q/%q, want 176843862716480/176844028768320", response.TaskID, response.FileID)
		}
		if response.VideoWidth == nil || *response.VideoWidth != 1280 || response.VideoHeight == nil || *response.VideoHeight != 720 {
			t.Fatalf("dimensions = %v x %v, want 1280 x 720", response.VideoWidth, response.VideoHeight)
		}
	})

	t.Run("empty task_id fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Video.GetTask(context.Background(), " ")
		if err == nil {
			t.Fatal("GetTask() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "task_id is empty") {
			t.Fatalf("GetTask() error = %v, want task_id validation error", err)
		}
	})

	t.Run("http 5xx returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"temporary unavailable"}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		_, err := client.Video.GetTask(context.Background(), "task_123")
		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("GetTask() error type = %T, want *protocol.APIError", err)
		}
		if apiErr.HTTPStatus != http.StatusServiceUnavailable {
			t.Fatalf("apiErr.HTTPStatus = %d, want %d", apiErr.HTTPStatus, http.StatusServiceUnavailable)
		}
	})

	t.Run("base_resp non-zero returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":1027,"status_msg":"unsafe output"}}`))
		}))
		defer srv.Close()

		client := newVideoTestClient(t, srv)
		_, err := client.Video.GetTask(context.Background(), "task_123")
		assertAPIStatus(t, err, 1027, "unsafe output")
	})

	t.Run("context canceled is preserved", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = client.Video.GetTask(ctx, "task_123")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("GetTask() error = %v, want context canceled", err)
		}
	})
}

func newVideoTestClient(t *testing.T, srv *httptest.Server) *Client {
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

func videoBoolPtr(value bool) *bool {
	return &value
}

func videoIntPtr(value int) *int {
	return &value
}
