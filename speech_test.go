package minimax

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GizClaw/minimax-go/internal/codec"
	"github.com/GizClaw/minimax-go/internal/protocol"
	"github.com/GizClaw/minimax-go/internal/transport"
)

func TestSpeechSynthesize(t *testing.T) {
	t.Parallel()

	t.Run("success returns audio bytes", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != defaultSpeechSynthesizePath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultSpeechSynthesizePath)
			}

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Trace-ID", "trace-speech-1")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"data":{"audio_hex":"48656c6c6f"}}`))
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

		resp, err := client.Speech.Synthesize(context.Background(), SpeechRequest{Text: "hello"})
		if err != nil {
			t.Fatalf("Synthesize() error = %v, want nil", err)
		}

		if len(resp.Audio) == 0 {
			t.Fatal("len(resp.Audio) = 0, want > 0")
		}

		if string(resp.Audio) != "Hello" {
			t.Fatalf("resp.Audio = %q, want %q", string(resp.Audio), "Hello")
		}

		if resp.ResponseMeta.TraceID != "trace-speech-1" {
			t.Fatalf("resp.ResponseMeta.TraceID = %q, want trace-speech-1", resp.ResponseMeta.TraceID)
		}
	})

	t.Run("volume zero is serialized in request", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll(r.Body) error = %v", err)
			}

			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("Unmarshal(request body) error = %v", err)
			}

			voiceSetting, ok := payload["voice_setting"].(map[string]any)
			if !ok {
				t.Fatalf("voice_setting missing or invalid type: %T", payload["voice_setting"])
			}

			volValue, exists := voiceSetting["vol"]
			if !exists {
				t.Fatalf("voice_setting.vol missing in payload: %v", voiceSetting)
			}

			volNumber, ok := volValue.(float64)
			if !ok {
				t.Fatalf("voice_setting.vol type = %T, want float64", volValue)
			}

			if volNumber != 0 {
				t.Fatalf("voice_setting.vol = %v, want 0", volNumber)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"data":{"audio_hex":"48656c6c6f"}}`))
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

		zero := 0.0
		resp, err := client.Speech.Synthesize(context.Background(), SpeechRequest{
			Text:    "hello",
			VoiceID: "English_Graceful_Lady",
			Vol:     &zero,
		})
		if err != nil {
			t.Fatalf("Synthesize() error = %v, want nil", err)
		}

		if string(resp.Audio) != "Hello" {
			t.Fatalf("resp.Audio = %q, want %q", string(resp.Audio), "Hello")
		}
	})

	t.Run("api error returns unified error model", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":2301,"status_msg":"invalid voice"}}`))
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

		_, err = client.Speech.Synthesize(context.Background(), SpeechRequest{Text: "hello"})
		if err == nil {
			t.Fatal("Synthesize() error = nil, want non-nil")
		}

		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("Synthesize() error type = %T, want *protocol.APIError", err)
		}

		if apiErr.StatusCode != 2301 || apiErr.StatusMsg != "invalid voice" {
			t.Fatalf("apiErr = %+v, want status_code=2301 status_msg=invalid voice", apiErr)
		}
	})

	t.Run("timeout canceled by context", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(120 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"data":{"audio_hex":"48656c6c6f"}}`))
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

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		_, err = client.Speech.Synthesize(ctx, SpeechRequest{Text: "hello"})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Synthesize() error = %v, want context deadline exceeded", err)
		}
	})

	t.Run("retry eventually success", func(t *testing.T) {
		t.Parallel()

		var attempts atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			current := attempts.Add(1)
			if current < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"error":"temporary unavailable"}`))
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"data":{"audio_hex":"48656c6c6f"}}`))
		}))
		defer srv.Close()

		client, err := NewClient(Config{
			BaseURL:    srv.URL,
			HTTPClient: srv.Client(),
			Retry: transport.RetryConfig{
				MaxAttempts: 3,
				BaseDelay:   time.Millisecond,
				MaxDelay:    time.Millisecond,
				Sleep: func(context.Context, time.Duration) error {
					return nil
				},
			},
		})
		if err != nil {
			t.Fatalf("NewClient() error = %v, want nil", err)
		}

		resp, err := client.Speech.Synthesize(context.Background(), SpeechRequest{Text: "hello"})
		if err != nil {
			t.Fatalf("Synthesize() error = %v, want nil", err)
		}

		if string(resp.Audio) != "Hello" {
			t.Fatalf("resp.Audio = %q, want Hello", string(resp.Audio))
		}

		if got := attempts.Load(); got != 3 {
			t.Fatalf("attempts = %d, want 3", got)
		}
	})

	t.Run("retry exhausted returns last retryable error", func(t *testing.T) {
		t.Parallel()

		var attempts atomic.Int32
		var sleepCalls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"temporary unavailable"}`))
		}))
		defer srv.Close()

		client, err := NewClient(Config{
			BaseURL:    srv.URL,
			HTTPClient: srv.Client(),
			Retry: transport.RetryConfig{
				MaxAttempts: 3,
				BaseDelay:   time.Millisecond,
				MaxDelay:    time.Millisecond,
				Sleep: func(context.Context, time.Duration) error {
					sleepCalls.Add(1)
					return nil
				},
			},
		})
		if err != nil {
			t.Fatalf("NewClient() error = %v, want nil", err)
		}

		_, err = client.Speech.Synthesize(context.Background(), SpeechRequest{Text: "hello"})
		if err == nil {
			t.Fatal("Synthesize() error = nil, want non-nil")
		}

		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("Synthesize() error type = %T, want *protocol.APIError", err)
		}

		if apiErr.HTTPStatus != http.StatusServiceUnavailable {
			t.Fatalf("apiErr.HTTPStatus = %d, want %d", apiErr.HTTPStatus, http.StatusServiceUnavailable)
		}

		if got := attempts.Load(); got != 3 {
			t.Fatalf("attempts = %d, want 3", got)
		}

		if got := sleepCalls.Load(); got != 2 {
			t.Fatalf("sleepCalls = %d, want 2", got)
		}
	})

	t.Run("invalid hex returns decode error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"data":{"audio_hex":"xyz"}}`))
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

		_, err = client.Speech.Synthesize(context.Background(), SpeechRequest{Text: "hello"})
		if !errors.Is(err, codec.ErrInvalidHexAudio) {
			t.Fatalf("Synthesize() error = %v, want codec.ErrInvalidHexAudio", err)
		}
	})

	t.Run("explicit context cancel", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(120 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"data":{"audio_hex":"48656c6c6f"}}`))
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

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = client.Speech.Synthesize(ctx, SpeechRequest{Text: "hello"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Synthesize() error = %v, want context canceled", err)
		}
	})
}

func TestSpeechStream(t *testing.T) {
	t.Parallel()

	t.Run("success reads multi chunk and done", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != defaultSpeechStreamPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultSpeechStreamPath)
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll(r.Body) error = %v", err)
			}

			var payload struct {
				Text         string `json:"text"`
				Model        string `json:"model"`
				Stream       bool   `json:"stream"`
				OutputFormat string `json:"output_format"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("Unmarshal(request body) error = %v", err)
			}

			if payload.Text != "hello" {
				t.Fatalf("payload.Text = %q, want hello", payload.Text)
			}

			if payload.Model != defaultSpeechModel {
				t.Fatalf("payload.Model = %q, want %q", payload.Model, defaultSpeechModel)
			}

			if !payload.Stream {
				t.Fatal("payload.Stream = false, want true")
			}

			if payload.OutputFormat != defaultSpeechOutputFormat {
				t.Fatalf("payload.OutputFormat = %q, want %q", payload.OutputFormat, defaultSpeechOutputFormat)
			}

			w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
			w.WriteHeader(http.StatusOK)

			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatal("response writer does not support flushing")
			}

			_, _ = w.Write([]byte(": keepalive\n\n"))
			flusher.Flush()

			_, _ = w.Write([]byte("event: meta\ndata: {}\n\n"))
			flusher.Flush()

			_, _ = w.Write([]byte("data: {\"data\":{\"audio_hex\":\"4865\"}}\n\n"))
			flusher.Flush()

			_, _ = w.Write([]byte("data: {\"data\":{\"audio_hex\":\"6c6c6f\"}}\n\n"))
			flusher.Flush()

			_, _ = w.Write([]byte("event: done\ndata: [DONE]\n\n"))
			flusher.Flush()
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

		stream, err := client.Speech.OpenStream(context.Background(), SpeechStreamRequest{Text: "hello"})
		if err != nil {
			t.Fatalf("OpenStream() error = %v, want nil", err)
		}

		chunk1, err := stream.Next()
		if err != nil {
			t.Fatalf("first Next() error = %v, want nil", err)
		}

		if chunk1.Done {
			t.Fatal("first chunk is done, want audio chunk")
		}

		if got := string(chunk1.Audio); got != "He" {
			t.Fatalf("first chunk audio = %q, want %q", got, "He")
		}

		chunk2, err := stream.Next()
		if err != nil {
			t.Fatalf("second Next() error = %v, want nil", err)
		}

		if chunk2.Done {
			t.Fatal("second chunk is done, want audio chunk")
		}

		if got := string(chunk2.Audio); got != "llo" {
			t.Fatalf("second chunk audio = %q, want %q", got, "llo")
		}

		doneChunk, err := stream.Next()
		if err != nil {
			t.Fatalf("done Next() error = %v, want nil", err)
		}

		if !doneChunk.Done {
			t.Fatalf("doneChunk.Done = false, want true: %+v", doneChunk)
		}

		if got := strings.ToLower(doneChunk.Event); got != "done" {
			t.Fatalf("doneChunk.Event = %q, want %q", doneChunk.Event, "done")
		}

		_, err = stream.Next()
		if !errors.Is(err, io.EOF) {
			t.Fatalf("next after done error = %v, want io.EOF", err)
		}

		if err := stream.Close(); err != nil {
			t.Fatalf("Close() error = %v, want nil", err)
		}

		if err := stream.Close(); err != nil {
			t.Fatalf("second Close() error = %v, want nil", err)
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

		_, err = client.Speech.OpenStream(context.Background(), SpeechStreamRequest{Text: "hello"})
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

	t.Run("unexpected non-stream content type should fail", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"}}`))
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

		_, err = client.Speech.OpenStream(context.Background(), SpeechStreamRequest{Text: "hello"})
		if err == nil {
			t.Fatal("OpenStream() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "unexpected stream content type") {
			t.Fatalf("OpenStream() error = %v, want unexpected stream content type", err)
		}
	})

	t.Run("unsupported output format is rejected before opening stream", func(t *testing.T) {
		t.Parallel()

		var requests atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("event: done\ndata: [DONE]\n\n"))
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

		_, err = client.Speech.OpenStream(context.Background(), SpeechStreamRequest{
			Text:         "hello",
			OutputFormat: "mp3",
		})
		if err == nil {
			t.Fatal("OpenStream() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "only \"hex\" is supported") {
			t.Fatalf("OpenStream() error = %v, want unsupported format error", err)
		}

		if got := requests.Load(); got != 0 {
			t.Fatalf("stream open requests = %d, want 0", got)
		}
	})

	t.Run("event status code error returns unified APIError", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("data: {\"status_code\":2301,\"status_msg\":\"stream event failed\"}\n\n"))
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

		stream, err := client.Speech.OpenStream(context.Background(), SpeechStreamRequest{Text: "hello"})
		if err != nil {
			t.Fatalf("OpenStream() error = %v, want nil", err)
		}
		defer stream.Close()

		_, err = stream.Next()
		if err == nil {
			t.Fatal("Next() error = nil, want non-nil")
		}

		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("Next() error type = %T, want *protocol.APIError", err)
		}

		if apiErr.StatusCode != 2301 {
			t.Fatalf("apiErr.StatusCode = %d, want 2301", apiErr.StatusCode)
		}

		if apiErr.StatusMsg != "stream event failed" {
			t.Fatalf("apiErr.StatusMsg = %q, want stream event failed", apiErr.StatusMsg)
		}
	})

	t.Run("invalid hex chunk returns decode error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("data: {\"data\":{\"audio_hex\":\"xyz\"}}\n\n"))
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

		stream, err := client.Speech.OpenStream(context.Background(), SpeechStreamRequest{Text: "hello"})
		if err != nil {
			t.Fatalf("OpenStream() error = %v, want nil", err)
		}
		defer stream.Close()

		_, err = stream.Next()
		if !errors.Is(err, codec.ErrInvalidHexAudio) {
			t.Fatalf("Next() error = %v, want codec.ErrInvalidHexAudio", err)
		}
	})

	t.Run("timeout canceled by context while opening stream", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(120 * time.Millisecond)
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("event: done\ndata: [DONE]\n\n"))
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

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		_, err = client.Speech.OpenStream(ctx, SpeechStreamRequest{Text: "hello"})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("OpenStream() error = %v, want context deadline exceeded", err)
		}
	})

	t.Run("explicit context cancel while opening stream", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("event: done\ndata: [DONE]\n\n"))
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

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = client.Speech.OpenStream(ctx, SpeechStreamRequest{Text: "hello"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("OpenStream() error = %v, want context canceled", err)
		}
	})

	t.Run("Stream alias delegates to OpenStream", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("event: done\ndata: [DONE]\n\n"))
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

		stream, err := client.Speech.Stream(context.Background(), SpeechStreamRequest{Text: "hello"})
		if err != nil {
			t.Fatalf("Stream() error = %v, want nil", err)
		}
		defer stream.Close()

		doneChunk, err := stream.Next()
		if err != nil {
			t.Fatalf("Next() error = %v, want nil", err)
		}

		if !doneChunk.Done {
			t.Fatalf("doneChunk.Done = false, want true: %+v", doneChunk)
		}
	})
}
