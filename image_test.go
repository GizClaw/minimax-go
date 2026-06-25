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

func TestImageGenerateTextToImage(t *testing.T) {
	t.Parallel()

	t.Run("success maps url response", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != defaultImageGenerationPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultImageGenerationPath)
			}

			var payload ImageTextToImageRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.Model != "image-01-live" || payload.Prompt != "A green circuit board on a clean desk" {
				t.Fatalf("payload model/prompt = %q/%q, want trimmed values", payload.Model, payload.Prompt)
			}
			if payload.Style == nil || payload.Style.StyleType != "watercolor" || payload.Style.StyleWeight == nil || *payload.Style.StyleWeight != 0.8 {
				t.Fatalf("payload.Style = %+v, want trimmed style with weight", payload.Style)
			}
			if payload.AspectRatio != "16:9" || payload.ResponseFormat != "url" {
				t.Fatalf("payload aspect/format = %q/%q, want 16:9/url", payload.AspectRatio, payload.ResponseFormat)
			}
			if payload.Width == nil || *payload.Width != 1280 || payload.Height == nil || *payload.Height != 720 {
				t.Fatalf("payload dimensions = %v x %v, want 1280 x 720", payload.Width, payload.Height)
			}
			if payload.Seed == nil || *payload.Seed != 42 || payload.N == nil || *payload.N != 2 {
				t.Fatalf("payload seed/n = %v/%v, want 42/2", payload.Seed, payload.N)
			}
			if payload.PromptOptimizer == nil || !*payload.PromptOptimizer {
				t.Fatalf("payload.PromptOptimizer = %v, want explicit true", payload.PromptOptimizer)
			}
			if payload.AIGCWatermark == nil || *payload.AIGCWatermark {
				t.Fatalf("payload.AIGCWatermark = %v, want explicit false", payload.AIGCWatermark)
			}

			w.Header().Set("X-Trace-ID", "trace-image-url")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"img_task_123","data":{"image_urls":["https://example.com/1.png","https://example.com/2.png"]},"metadata":{"success_count":"2","failed_count":"0"},"extra":"kept","base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newImageTestClient(t, srv)
		response, err := client.Image.GenerateTextToImage(context.Background(), ImageTextToImageRequest{
			Model:           " image-01-live ",
			Prompt:          " A green circuit board on a clean desk ",
			Style:           &ImageStyle{StyleType: " watercolor ", StyleWeight: new(0.8)},
			AspectRatio:     " 16:9 ",
			Width:           new(1280),
			Height:          new(720),
			ResponseFormat:  " url ",
			Seed:            new(int64(42)),
			N:               new(2),
			PromptOptimizer: new(true),
			AIGCWatermark:   new(false),
		})
		if err != nil {
			t.Fatalf("GenerateTextToImage() error = %v, want nil", err)
		}
		if response.ID != "img_task_123" {
			t.Fatalf("response.ID = %q, want img_task_123", response.ID)
		}
		if len(response.ImageURLs) != 2 || response.ImageURLs[0] != "https://example.com/1.png" || response.ImageURLs[1] != "https://example.com/2.png" {
			t.Fatalf("response.ImageURLs = %+v, want two URLs", response.ImageURLs)
		}
		if len(response.ImageBase64) != 0 {
			t.Fatalf("response.ImageBase64 = %+v, want empty", response.ImageBase64)
		}
		if response.Metadata.SuccessCount == nil || *response.Metadata.SuccessCount != 2 {
			t.Fatalf("SuccessCount = %v, want 2", response.Metadata.SuccessCount)
		}
		if response.Metadata.FailedCount == nil || *response.Metadata.FailedCount != 0 {
			t.Fatalf("FailedCount = %v, want 0", response.Metadata.FailedCount)
		}
		if response.ResponseMeta.TraceID != "trace-image-url" {
			t.Fatalf("TraceID = %q, want trace-image-url", response.ResponseMeta.TraceID)
		}
		if _, ok := response.Raw["extra"]; !ok {
			t.Fatalf("response.Raw missing extra field: %+v", response.Raw)
		}
	})

	t.Run("success maps base64 response", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"img_task_base64","data":{"image_base64":["ZmFrZS1wbmc="]},"metadata":{"success_count":1,"failed_count":0},"base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newImageTestClient(t, srv)
		response, err := client.Image.GenerateTextToImage(context.Background(), ImageTextToImageRequest{
			Model:          "image-01",
			Prompt:         "tiny robot icon",
			ResponseFormat: "base64",
		})
		if err != nil {
			t.Fatalf("GenerateTextToImage() error = %v, want nil", err)
		}
		if response.ID != "img_task_base64" || len(response.ImageBase64) != 1 || response.ImageBase64[0] != "ZmFrZS1wbmc=" {
			t.Fatalf("response = %+v, want base64 image response", response)
		}
	})

	t.Run("empty model fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Image.GenerateTextToImage(context.Background(), ImageTextToImageRequest{Prompt: "hello"})
		if err == nil || !strings.Contains(err.Error(), "model is empty") {
			t.Fatalf("GenerateTextToImage() error = %v, want model validation error", err)
		}
	})

	t.Run("empty prompt fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Image.GenerateTextToImage(context.Background(), ImageTextToImageRequest{Model: "image-01"})
		if err == nil || !strings.Contains(err.Error(), "prompt is empty") {
			t.Fatalf("GenerateTextToImage() error = %v, want prompt validation error", err)
		}
	})

	t.Run("one-sided dimensions fail fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Image.GenerateTextToImage(context.Background(), ImageTextToImageRequest{
			Model:  "image-01",
			Prompt: "hello",
			Width:  new(1024),
		})
		if err == nil || !strings.Contains(err.Error(), "width and height must be provided together") {
			t.Fatalf("GenerateTextToImage() error = %v, want dimension pair validation error", err)
		}
	})

	t.Run("invalid dimension bounds fail fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Image.GenerateTextToImage(context.Background(), ImageTextToImageRequest{
			Model:  "image-01",
			Prompt: "hello",
			Width:  new(511),
			Height: new(1024),
		})
		if err == nil || !strings.Contains(err.Error(), "width must be between 512 and 2048") {
			t.Fatalf("GenerateTextToImage() error = %v, want dimension bound validation error", err)
		}
	})

	t.Run("invalid dimension multiple fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Image.GenerateTextToImage(context.Background(), ImageTextToImageRequest{
			Model:  "image-01",
			Prompt: "hello",
			Width:  new(1025),
			Height: new(1024),
		})
		if err == nil || !strings.Contains(err.Error(), "width must be a multiple of 8") {
			t.Fatalf("GenerateTextToImage() error = %v, want dimension multiple validation error", err)
		}
	})

	t.Run("invalid n fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Image.GenerateTextToImage(context.Background(), ImageTextToImageRequest{
			Model:  "image-01",
			Prompt: "hello",
			N:      new(10),
		})
		if err == nil || !strings.Contains(err.Error(), "n must be between 1 and 9") {
			t.Fatalf("GenerateTextToImage() error = %v, want n validation error", err)
		}
	})

	t.Run("invalid style weight fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Image.GenerateTextToImage(context.Background(), ImageTextToImageRequest{
			Model:  "image-01-live",
			Prompt: "hello",
			Style:  &ImageStyle{StyleType: "watercolor", StyleWeight: new(1.5)},
		})
		if err == nil || !strings.Contains(err.Error(), "style_weight") {
			t.Fatalf("GenerateTextToImage() error = %v, want style weight validation error", err)
		}
	})

	t.Run("invalid response format fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Image.GenerateTextToImage(context.Background(), ImageTextToImageRequest{
			Model:          "image-01",
			Prompt:         "hello",
			ResponseFormat: "binary",
		})
		if err == nil || !strings.Contains(err.Error(), "response_format") {
			t.Fatalf("GenerateTextToImage() error = %v, want response format validation error", err)
		}
	})

	t.Run("http error returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		client := newImageTestClient(t, srv)
		_, err := client.Image.GenerateTextToImage(context.Background(), ImageTextToImageRequest{
			Model:  "image-01",
			Prompt: "hello",
		})
		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("error type = %T, want *protocol.APIError", err)
		}
		if apiErr.HTTPStatus != http.StatusServiceUnavailable {
			t.Fatalf("apiErr.HTTPStatus = %d, want 503", apiErr.HTTPStatus)
		}
	})

	t.Run("base_resp non-zero returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":1026,"status_msg":"sensitive prompt"}}`))
		}))
		defer srv.Close()

		client := newImageTestClient(t, srv)
		_, err := client.Image.GenerateTextToImage(context.Background(), ImageTextToImageRequest{
			Model:  "image-01",
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
		_, err = client.Image.GenerateTextToImage(ctx, ImageTextToImageRequest{
			Model:  "image-01",
			Prompt: "hello",
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("GenerateTextToImage() error = %v, want context canceled", err)
		}
	})
}

func TestImageGenerateImageToImage(t *testing.T) {
	t.Parallel()

	t.Run("success maps url response", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != defaultImageGenerationPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultImageGenerationPath)
			}

			var payload ImageImageToImageRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.Model != "image-01-live" || payload.Prompt != "A portrait in a library window" {
				t.Fatalf("payload model/prompt = %q/%q, want trimmed values", payload.Model, payload.Prompt)
			}
			if len(payload.SubjectReferences) != 1 {
				t.Fatalf("payload.SubjectReferences = %+v, want one reference", payload.SubjectReferences)
			}
			reference := payload.SubjectReferences[0]
			if reference.Type != "character" || reference.ImageFile != "https://example.com/reference.png" {
				t.Fatalf("reference = %+v, want trimmed character reference", reference)
			}
			if payload.Style == nil || payload.Style.StyleType != "watercolor" || payload.Style.StyleWeight == nil || *payload.Style.StyleWeight != 0.6 {
				t.Fatalf("payload.Style = %+v, want trimmed style with weight", payload.Style)
			}
			if payload.AspectRatio != "16:9" || payload.ResponseFormat != "url" {
				t.Fatalf("payload aspect/format = %q/%q, want 16:9/url", payload.AspectRatio, payload.ResponseFormat)
			}
			if payload.Width == nil || *payload.Width != 1280 || payload.Height == nil || *payload.Height != 720 {
				t.Fatalf("payload dimensions = %v x %v, want 1280 x 720", payload.Width, payload.Height)
			}
			if payload.Seed == nil || *payload.Seed != 77 || payload.N == nil || *payload.N != 2 {
				t.Fatalf("payload seed/n = %v/%v, want 77/2", payload.Seed, payload.N)
			}
			if payload.PromptOptimizer == nil || *payload.PromptOptimizer {
				t.Fatalf("payload.PromptOptimizer = %v, want explicit false", payload.PromptOptimizer)
			}
			if payload.AIGCWatermark == nil || !*payload.AIGCWatermark {
				t.Fatalf("payload.AIGCWatermark = %v, want explicit true", payload.AIGCWatermark)
			}

			w.Header().Set("X-Trace-ID", "trace-image-i2i-url")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"img_i2i_123","data":{"image_urls":["https://example.com/i2i-1.png"]},"metadata":{"success_count":"1","failed_count":"0"},"extra":"kept","base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newImageTestClient(t, srv)
		response, err := client.Image.GenerateImageToImage(context.Background(), ImageImageToImageRequest{
			Model:  " image-01-live ",
			Prompt: " A portrait in a library window ",
			SubjectReferences: []ImageSubjectReference{{
				Type:      " character ",
				ImageFile: " https://example.com/reference.png ",
			}},
			Style:           &ImageStyle{StyleType: " watercolor ", StyleWeight: new(0.6)},
			AspectRatio:     " 16:9 ",
			Width:           new(1280),
			Height:          new(720),
			ResponseFormat:  " url ",
			Seed:            new(int64(77)),
			N:               new(2),
			PromptOptimizer: new(false),
			AIGCWatermark:   new(true),
		})
		if err != nil {
			t.Fatalf("GenerateImageToImage() error = %v, want nil", err)
		}
		if response.ID != "img_i2i_123" {
			t.Fatalf("response.ID = %q, want img_i2i_123", response.ID)
		}
		if len(response.ImageURLs) != 1 || response.ImageURLs[0] != "https://example.com/i2i-1.png" {
			t.Fatalf("response.ImageURLs = %+v, want one URL", response.ImageURLs)
		}
		if response.Metadata.SuccessCount == nil || *response.Metadata.SuccessCount != 1 {
			t.Fatalf("SuccessCount = %v, want 1", response.Metadata.SuccessCount)
		}
		if response.Metadata.FailedCount == nil || *response.Metadata.FailedCount != 0 {
			t.Fatalf("FailedCount = %v, want 0", response.Metadata.FailedCount)
		}
		if response.ResponseMeta.TraceID != "trace-image-i2i-url" {
			t.Fatalf("TraceID = %q, want trace-image-i2i-url", response.ResponseMeta.TraceID)
		}
		if _, ok := response.Raw["extra"]; !ok {
			t.Fatalf("response.Raw missing extra field: %+v", response.Raw)
		}
	})

	t.Run("success maps base64 response", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"img_i2i_base64","data":{"image_base64":["ZmFrZS1pMmktcG5n"]},"metadata":{"success_count":1,"failed_count":0},"base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newImageTestClient(t, srv)
		response, err := client.Image.GenerateImageToImage(context.Background(), ImageImageToImageRequest{
			Model:          "image-01",
			Prompt:         "paint this subject",
			ResponseFormat: "base64",
			SubjectReferences: []ImageSubjectReference{{
				Type:      "character",
				ImageFile: "data:image/png;base64,AAAA",
			}},
		})
		if err != nil {
			t.Fatalf("GenerateImageToImage() error = %v, want nil", err)
		}
		if response.ID != "img_i2i_base64" || len(response.ImageBase64) != 1 || response.ImageBase64[0] != "ZmFrZS1pMmktcG5n" {
			t.Fatalf("response = %+v, want base64 image response", response)
		}
	})

	t.Run("empty model fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Image.GenerateImageToImage(context.Background(), ImageImageToImageRequest{
			Prompt: "hello",
			SubjectReferences: []ImageSubjectReference{{
				Type:      "character",
				ImageFile: "https://example.com/reference.png",
			}},
		})
		if err == nil || !strings.Contains(err.Error(), "model is empty") {
			t.Fatalf("GenerateImageToImage() error = %v, want model validation error", err)
		}
	})

	t.Run("empty prompt fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Image.GenerateImageToImage(context.Background(), ImageImageToImageRequest{
			Model: "image-01",
			SubjectReferences: []ImageSubjectReference{{
				Type:      "character",
				ImageFile: "https://example.com/reference.png",
			}},
		})
		if err == nil || !strings.Contains(err.Error(), "prompt is empty") {
			t.Fatalf("GenerateImageToImage() error = %v, want prompt validation error", err)
		}
	})

	t.Run("missing subject references fail fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Image.GenerateImageToImage(context.Background(), ImageImageToImageRequest{
			Model:  "image-01",
			Prompt: "hello",
		})
		if err == nil || !strings.Contains(err.Error(), "subject_reference is empty") {
			t.Fatalf("GenerateImageToImage() error = %v, want subject reference validation error", err)
		}
	})

	t.Run("empty subject reference type fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Image.GenerateImageToImage(context.Background(), ImageImageToImageRequest{
			Model:  "image-01",
			Prompt: "hello",
			SubjectReferences: []ImageSubjectReference{{
				ImageFile: "https://example.com/reference.png",
			}},
		})
		if err == nil || !strings.Contains(err.Error(), "subject_reference[0].type is empty") {
			t.Fatalf("GenerateImageToImage() error = %v, want subject reference type validation error", err)
		}
	})

	t.Run("empty subject reference image file fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Image.GenerateImageToImage(context.Background(), ImageImageToImageRequest{
			Model:  "image-01",
			Prompt: "hello",
			SubjectReferences: []ImageSubjectReference{{
				Type: "character",
			}},
		})
		if err == nil || !strings.Contains(err.Error(), "subject_reference[0].image_file is empty") {
			t.Fatalf("GenerateImageToImage() error = %v, want subject reference image file validation error", err)
		}
	})

	t.Run("one-sided dimensions fail fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Image.GenerateImageToImage(context.Background(), ImageImageToImageRequest{
			Model:  "image-01",
			Prompt: "hello",
			SubjectReferences: []ImageSubjectReference{{
				Type:      "character",
				ImageFile: "https://example.com/reference.png",
			}},
			Width: new(1024),
		})
		if err == nil || !strings.Contains(err.Error(), "width and height must be provided together") {
			t.Fatalf("GenerateImageToImage() error = %v, want dimension pair validation error", err)
		}
	})

	t.Run("invalid n fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Image.GenerateImageToImage(context.Background(), ImageImageToImageRequest{
			Model:  "image-01",
			Prompt: "hello",
			SubjectReferences: []ImageSubjectReference{{
				Type:      "character",
				ImageFile: "https://example.com/reference.png",
			}},
			N: new(10),
		})
		if err == nil || !strings.Contains(err.Error(), "n must be between 1 and 9") {
			t.Fatalf("GenerateImageToImage() error = %v, want n validation error", err)
		}
	})

	t.Run("invalid response format fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		_, err = client.Image.GenerateImageToImage(context.Background(), ImageImageToImageRequest{
			Model:  "image-01",
			Prompt: "hello",
			SubjectReferences: []ImageSubjectReference{{
				Type:      "character",
				ImageFile: "https://example.com/reference.png",
			}},
			ResponseFormat: "binary",
		})
		if err == nil || !strings.Contains(err.Error(), "response_format") {
			t.Fatalf("GenerateImageToImage() error = %v, want response format validation error", err)
		}
	})

	t.Run("http error returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		client := newImageTestClient(t, srv)
		_, err := client.Image.GenerateImageToImage(context.Background(), ImageImageToImageRequest{
			Model:  "image-01",
			Prompt: "hello",
			SubjectReferences: []ImageSubjectReference{{
				Type:      "character",
				ImageFile: "https://example.com/reference.png",
			}},
		})
		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("error type = %T, want *protocol.APIError", err)
		}
		if apiErr.HTTPStatus != http.StatusServiceUnavailable {
			t.Fatalf("apiErr.HTTPStatus = %d, want 503", apiErr.HTTPStatus)
		}
	})

	t.Run("base_resp non-zero returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":2013,"status_msg":"invalid image"}}`))
		}))
		defer srv.Close()

		client := newImageTestClient(t, srv)
		_, err := client.Image.GenerateImageToImage(context.Background(), ImageImageToImageRequest{
			Model:  "image-01",
			Prompt: "hello",
			SubjectReferences: []ImageSubjectReference{{
				Type:      "character",
				ImageFile: "https://example.com/reference.png",
			}},
		})
		assertAPIStatus(t, err, 2013, "invalid image")
	})

	t.Run("context canceled is preserved", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = client.Image.GenerateImageToImage(ctx, ImageImageToImageRequest{
			Model:  "image-01",
			Prompt: "hello",
			SubjectReferences: []ImageSubjectReference{{
				Type:      "character",
				ImageFile: "https://example.com/reference.png",
			}},
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("GenerateImageToImage() error = %v, want context canceled", err)
		}
	})
}

func newImageTestClient(t *testing.T, srv *httptest.Server) *Client {
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
