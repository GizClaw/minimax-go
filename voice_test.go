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

	"github.com/GizClaw/minimax-go/internal/protocol"
	"github.com/GizClaw/minimax-go/internal/transport"
)

func TestListVoices(t *testing.T) {
	t.Parallel()

	t.Run("success with default request", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}

			if r.URL.Path != defaultVoiceListPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultVoiceListPath)
			}

			if got := r.URL.Query().Get("voice_type"); got != defaultVoiceType {
				t.Fatalf("query.voice_type = %q, want %q", got, defaultVoiceType)
			}

			var payload listVoicesWireRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode(request body) error = %v", err)
			}

			if payload.VoiceType != defaultVoiceType {
				t.Fatalf("payload.voice_type = %q, want %q", payload.VoiceType, defaultVoiceType)
			}

			if payload.PageSize != nil {
				t.Fatalf("payload.page_size = %d, want nil", *payload.PageSize)
			}

			if payload.PageToken != "" {
				t.Fatalf("payload.page_token = %q, want empty", payload.PageToken)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"voices":[{"voice_id":"voice-system-1","voice_name":"calm narrator","description":["calm"],"created_time":"2026-03-01","voice_type":"system","gender":"female"}],"next_page_token":"cursor-2","has_more":true,"request_id":"req-1"}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.Voice.ListVoices(context.Background(), nil)
		if err != nil {
			t.Fatalf("ListVoices() error = %v, want nil", err)
		}

		if got := len(resp.Voices); got != 1 {
			t.Fatalf("len(resp.Voices) = %d, want 1", got)
		}

		if resp.NextPageToken != "cursor-2" {
			t.Fatalf("resp.NextPageToken = %q, want %q", resp.NextPageToken, "cursor-2")
		}

		if !resp.HasMore {
			t.Fatal("resp.HasMore = false, want true")
		}

		voice := resp.Voices[0]
		if voice.VoiceID != "voice-system-1" || voice.VoiceType != "system" {
			t.Fatalf("voice = %+v, want voice_id=voice-system-1 voice_type=system", voice)
		}

		if _, ok := voice.Raw["gender"]; !ok {
			t.Fatalf("voice.Raw = %v, want gender field", voice.Raw)
		}

		if _, ok := resp.Raw["request_id"]; !ok {
			t.Fatalf("resp.Raw = %v, want request_id field", resp.Raw)
		}
	})

	t.Run("empty list returns empty slice", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"voices":[]}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.Voice.ListVoices(context.Background(), &ListVoicesRequest{VoiceType: "system"})
		if err != nil {
			t.Fatalf("ListVoices() error = %v, want nil", err)
		}

		if resp.Voices == nil {
			t.Fatal("resp.Voices = nil, want empty slice")
		}

		if got := len(resp.Voices); got != 0 {
			t.Fatalf("len(resp.Voices) = %d, want 0", got)
		}
	})

	t.Run("pagination and filter are forwarded", func(t *testing.T) {
		t.Parallel()

		pageSize := 25
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			if query.Get("voice_type") != "voice_cloning" {
				t.Fatalf("query.voice_type = %q, want %q", query.Get("voice_type"), "voice_cloning")
			}

			if query.Get("page_size") != "25" {
				t.Fatalf("query.page_size = %q, want %q", query.Get("page_size"), "25")
			}

			if query.Get("page_token") != "cursor-2" {
				t.Fatalf("query.page_token = %q, want %q", query.Get("page_token"), "cursor-2")
			}

			var payload listVoicesWireRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode(request body) error = %v", err)
			}

			if payload.VoiceType != "voice_cloning" {
				t.Fatalf("payload.voice_type = %q, want %q", payload.VoiceType, "voice_cloning")
			}

			if payload.PageSize == nil || *payload.PageSize != pageSize {
				t.Fatalf("payload.page_size = %v, want %d", payload.PageSize, pageSize)
			}

			if payload.PageToken != "cursor-2" {
				t.Fatalf("payload.page_token = %q, want %q", payload.PageToken, "cursor-2")
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"voices":[{"voice_id":"clone-1"}],"next_page_token":"cursor-3","has_more":true}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.Voice.ListVoices(context.Background(), &ListVoicesRequest{
			VoiceType: "voice_cloning",
			PageSize:  &pageSize,
			PageToken: "cursor-2",
		})
		if err != nil {
			t.Fatalf("ListVoices() error = %v, want nil", err)
		}

		if resp.NextPageToken != "cursor-3" || !resp.HasMore {
			t.Fatalf("resp = %+v, want next_page_token=cursor-3 has_more=true", resp)
		}
	})

	t.Run("legacy response shape is normalized", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"system_voice":[{"voice_id":"sys-1","voice_name":"sys"}],"voice_cloning":[{"voice_id":"clone-1"}],"voice_generation":[{"voice_id":"gen-1"}],"request_id":"legacy-1"}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.Voice.ListVoices(context.Background(), &ListVoicesRequest{VoiceType: "all"})
		if err != nil {
			t.Fatalf("ListVoices() error = %v, want nil", err)
		}

		if got := len(resp.Voices); got != 3 {
			t.Fatalf("len(resp.Voices) = %d, want 3", got)
		}

		if resp.Voices[0].VoiceType != "system" || resp.Voices[1].VoiceType != "voice_cloning" || resp.Voices[2].VoiceType != "voice_generation" {
			t.Fatalf("voice types = [%s %s %s], want [system voice_cloning voice_generation]", resp.Voices[0].VoiceType, resp.Voices[1].VoiceType, resp.Voices[2].VoiceType)
		}

		if _, ok := resp.Raw["request_id"]; !ok {
			t.Fatalf("resp.Raw = %v, want request_id field", resp.Raw)
		}
	})

	t.Run("http error returns unified APIError", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		_, err := client.Voice.ListVoices(context.Background(), &ListVoicesRequest{VoiceType: "all"})
		if err == nil {
			t.Fatal("ListVoices() error = nil, want non-nil")
		}

		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("ListVoices() error type = %T, want *protocol.APIError", err)
		}

		if apiErr.HTTPStatus != http.StatusUnauthorized {
			t.Fatalf("apiErr.HTTPStatus = %d, want %d", apiErr.HTTPStatus, http.StatusUnauthorized)
		}
	})

	t.Run("http 500 returns unified APIError", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal"}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		_, err := client.Voice.ListVoices(context.Background(), &ListVoicesRequest{VoiceType: "all"})
		if err == nil {
			t.Fatal("ListVoices() error = nil, want non-nil")
		}

		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("ListVoices() error type = %T, want *protocol.APIError", err)
		}

		if apiErr.HTTPStatus != http.StatusInternalServerError {
			t.Fatalf("apiErr.HTTPStatus = %d, want %d", apiErr.HTTPStatus, http.StatusInternalServerError)
		}
	})

	t.Run("base_resp error returns unified APIError", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":2013,"status_msg":"invalid voice_type"}}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		_, err := client.Voice.ListVoices(context.Background(), &ListVoicesRequest{VoiceType: "invalid"})
		if err == nil {
			t.Fatal("ListVoices() error = nil, want non-nil")
		}

		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("ListVoices() error type = %T, want *protocol.APIError", err)
		}

		if apiErr.StatusCode != 2013 || apiErr.StatusMsg != "invalid voice_type" {
			t.Fatalf("apiErr = %+v, want status_code=2013 status_msg=invalid voice_type", apiErr)
		}
	})

	t.Run("timeout canceled by context", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(120 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"voices":[]}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		_, err := client.Voice.ListVoices(ctx, &ListVoicesRequest{VoiceType: "all"})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("ListVoices() error = %v, want context deadline exceeded", err)
		}
	})
}

func TestVoiceListVoicesValidation(t *testing.T) {
	t.Parallel()

	t.Run("negative page size is rejected", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v, want nil", err)
		}

		negative := -1
		_, err = client.Voice.ListVoices(context.Background(), &ListVoicesRequest{
			VoiceType: "all",
			PageSize:  &negative,
		})
		if err == nil {
			t.Fatal("ListVoices() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "page_size") {
			t.Fatalf("ListVoices() error = %v, want page_size validation", err)
		}
	})

	t.Run("nil service returns initialization error", func(t *testing.T) {
		t.Parallel()

		var service *VoiceService
		_, err := service.ListVoices(context.Background(), nil)
		if err == nil {
			t.Fatal("ListVoices() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "not initialized") {
			t.Fatalf("ListVoices() error = %v, want initialization error", err)
		}
	})
}

func TestDesignVoice(t *testing.T) {
	t.Parallel()

	t.Run("success with required fields", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}

			if r.URL.Path != defaultVoiceDesignPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultVoiceDesignPath)
			}

			var payload designVoiceWireRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode(request body) error = %v", err)
			}

			if payload.Prompt != "energetic host" {
				t.Fatalf("payload.prompt = %q, want %q", payload.Prompt, "energetic host")
			}

			if payload.PreviewText != "hello everyone" {
				t.Fatalf("payload.preview_text = %q, want %q", payload.PreviewText, "hello everyone")
			}

			if payload.VoiceID != "" {
				t.Fatalf("payload.voice_id = %q, want empty", payload.VoiceID)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"voice_id":"voice-designed-1","trial_audio":"68656c6c6f","request_id":"req-design-1"}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.Voice.DesignVoice(context.Background(), &DesignVoiceRequest{
			Prompt:      " energetic host ",
			PreviewText: " hello everyone ",
		})
		if err != nil {
			t.Fatalf("DesignVoice() error = %v, want nil", err)
		}

		if resp.VoiceID != "voice-designed-1" {
			t.Fatalf("resp.VoiceID = %q, want %q", resp.VoiceID, "voice-designed-1")
		}

		if resp.TrialAudio != "68656c6c6f" {
			t.Fatalf("resp.TrialAudio = %q, want %q", resp.TrialAudio, "68656c6c6f")
		}

		if _, ok := resp.Raw["request_id"]; !ok {
			t.Fatalf("resp.Raw = %v, want request_id field", resp.Raw)
		}
	})

	t.Run("success with optional voice_id", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload designVoiceWireRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode(request body) error = %v", err)
			}

			if payload.VoiceID != "custom-voice-1" {
				t.Fatalf("payload.voice_id = %q, want %q", payload.VoiceID, "custom-voice-1")
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"custom_voice_id":"custom-voice-1","preview_audio":"70726576696577"}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.Voice.DesignVoice(context.Background(), &DesignVoiceRequest{
			Prompt:      "warm narrator",
			PreviewText: "preview",
			VoiceID:     " custom-voice-1 ",
		})
		if err != nil {
			t.Fatalf("DesignVoice() error = %v, want nil", err)
		}

		if resp.VoiceID != "custom-voice-1" {
			t.Fatalf("resp.VoiceID = %q, want %q", resp.VoiceID, "custom-voice-1")
		}

		if resp.TrialAudio != "70726576696577" {
			t.Fatalf("resp.TrialAudio = %q, want %q", resp.TrialAudio, "70726576696577")
		}
	})

	t.Run("missing prompt fails fast", func(t *testing.T) {
		t.Parallel()

		var requests atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			requests.Add(1)
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		_, err := client.Voice.DesignVoice(context.Background(), &DesignVoiceRequest{
			Prompt:      "   ",
			PreviewText: "hello",
		})
		if err == nil {
			t.Fatal("DesignVoice() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "prompt") {
			t.Fatalf("DesignVoice() error = %v, want prompt validation", err)
		}

		if got := requests.Load(); got != 0 {
			t.Fatalf("requests = %d, want 0", got)
		}
	})

	t.Run("missing preview_text fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		_, err = client.Voice.DesignVoice(context.Background(), &DesignVoiceRequest{Prompt: "hello"})
		if err == nil {
			t.Fatal("DesignVoice() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "preview_text") {
			t.Fatalf("DesignVoice() error = %v, want preview_text validation", err)
		}
	})

	t.Run("nil request fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		_, err = client.Voice.DesignVoice(context.Background(), nil)
		if err == nil {
			t.Fatal("DesignVoice() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "request is nil") {
			t.Fatalf("DesignVoice() error = %v, want request is nil", err)
		}
	})

	t.Run("http error returns unified APIError", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal"}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		_, err := client.Voice.DesignVoice(context.Background(), &DesignVoiceRequest{
			Prompt:      "hello",
			PreviewText: "world",
		})
		if err == nil {
			t.Fatal("DesignVoice() error = nil, want non-nil")
		}

		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("DesignVoice() error type = %T, want *protocol.APIError", err)
		}

		if apiErr.HTTPStatus != http.StatusInternalServerError {
			t.Fatalf("apiErr.HTTPStatus = %d, want %d", apiErr.HTTPStatus, http.StatusInternalServerError)
		}
	})

	t.Run("base_resp error returns unified APIError", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":2013,"status_msg":"invalid prompt"}}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		_, err := client.Voice.DesignVoice(context.Background(), &DesignVoiceRequest{
			Prompt:      "hello",
			PreviewText: "world",
		})
		if err == nil {
			t.Fatal("DesignVoice() error = nil, want non-nil")
		}

		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("DesignVoice() error type = %T, want *protocol.APIError", err)
		}

		if apiErr.StatusCode != 2013 || apiErr.StatusMsg != "invalid prompt" {
			t.Fatalf("apiErr = %+v, want status_code=2013 status_msg=invalid prompt", apiErr)
		}
	})

	t.Run("timeout canceled by context", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(120 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"voice_id":"v1"}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		_, err := client.Voice.DesignVoice(ctx, &DesignVoiceRequest{
			Prompt:      "hello",
			PreviewText: "world",
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("DesignVoice() error = %v, want context deadline exceeded", err)
		}
	})

	t.Run("explicit context cancel", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = client.Voice.DesignVoice(ctx, &DesignVoiceRequest{
			Prompt:      "hello",
			PreviewText: "world",
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("DesignVoice() error = %v, want context canceled", err)
		}
	})
}

func TestCloneVoice(t *testing.T) {
	t.Parallel()

	t.Run("success with audio_url", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}

			if r.URL.Path != defaultVoiceClonePath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultVoiceClonePath)
			}

			var payload cloneVoiceWireRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode(request body) error = %v", err)
			}

			if payload.VoiceID != "clone-voice-1" {
				t.Fatalf("payload.voice_id = %q, want %q", payload.VoiceID, "clone-voice-1")
			}

			if payload.AudioURL != "https://cdn.example.com/audio.wav" {
				t.Fatalf("payload.audio_url = %q, want %q", payload.AudioURL, "https://cdn.example.com/audio.wav")
			}

			if payload.FileID != "" {
				t.Fatalf("payload.file_id = %q, want empty", payload.FileID)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"voice_id":"clone-voice-1","demo_audio":"https://cdn.example.com/demo.mp3","request_id":"req-clone-1"}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.Voice.CloneVoice(context.Background(), &CloneVoiceRequest{
			VoiceID:  " clone-voice-1 ",
			AudioURL: " https://cdn.example.com/audio.wav ",
		})
		if err != nil {
			t.Fatalf("CloneVoice() error = %v, want nil", err)
		}

		if resp.VoiceID != "clone-voice-1" {
			t.Fatalf("resp.VoiceID = %q, want %q", resp.VoiceID, "clone-voice-1")
		}

		if resp.DemoAudio != "https://cdn.example.com/demo.mp3" {
			t.Fatalf("resp.DemoAudio = %q, want %q", resp.DemoAudio, "https://cdn.example.com/demo.mp3")
		}

		if _, ok := resp.Raw["request_id"]; !ok {
			t.Fatalf("resp.Raw = %v, want request_id field", resp.Raw)
		}
	})

	t.Run("success with file_id", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload cloneVoiceWireRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode(request body) error = %v", err)
			}

			if payload.FileID != "file_123" {
				t.Fatalf("payload.file_id = %q, want %q", payload.FileID, "file_123")
			}

			if payload.AudioURL != "" {
				t.Fatalf("payload.audio_url = %q, want empty", payload.AudioURL)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"trial_audio":"https://cdn.example.com/preview.wav"}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.Voice.CloneVoice(context.Background(), &CloneVoiceRequest{
			VoiceID: "clone-file-voice",
			FileID:  " file_123 ",
		})
		if err != nil {
			t.Fatalf("CloneVoice() error = %v, want nil", err)
		}

		if resp.VoiceID != "clone-file-voice" {
			t.Fatalf("resp.VoiceID = %q, want %q", resp.VoiceID, "clone-file-voice")
		}

		if resp.DemoAudio != "https://cdn.example.com/preview.wav" {
			t.Fatalf("resp.DemoAudio = %q, want %q", resp.DemoAudio, "https://cdn.example.com/preview.wav")
		}
	})

	t.Run("pure numeric file_id is encoded as JSON number", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll(r.Body) error = %v", err)
			}

			var payload map[string]json.RawMessage
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("Unmarshal(request body) error = %v", err)
			}

			rawFileID, ok := payload["file_id"]
			if !ok {
				t.Fatalf("request payload missing file_id: %s", string(body))
			}

			if got := strings.TrimSpace(string(rawFileID)); got != "372103253696905" {
				t.Fatalf("raw file_id token = %q, want %q", got, "372103253696905")
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"voice_id":"clone-numeric-file-id"}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.Voice.CloneVoice(context.Background(), &CloneVoiceRequest{
			VoiceID: "clone-numeric-file-id",
			FileID:  "372103253696905",
		})
		if err != nil {
			t.Fatalf("CloneVoice() error = %v, want nil", err)
		}

		if resp.VoiceID != "clone-numeric-file-id" {
			t.Fatalf("resp.VoiceID = %q, want %q", resp.VoiceID, "clone-numeric-file-id")
		}
	})

	t.Run("leading-zero file_id is encoded as JSON string", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll(r.Body) error = %v", err)
			}

			var payload map[string]json.RawMessage
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("Unmarshal(request body) error = %v", err)
			}

			rawFileID, ok := payload["file_id"]
			if !ok {
				t.Fatalf("request payload missing file_id: %s", string(body))
			}

			if got := strings.TrimSpace(string(rawFileID)); got != `"00123"` {
				t.Fatalf("raw file_id token = %q, want %q", got, `"00123"`)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"voice_id":"clone-leading-zero"}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.Voice.CloneVoice(context.Background(), &CloneVoiceRequest{
			VoiceID: "clone-leading-zero",
			FileID:  "00123",
		})
		if err != nil {
			t.Fatalf("CloneVoice() error = %v, want nil", err)
		}

		if resp.VoiceID != "clone-leading-zero" {
			t.Fatalf("resp.VoiceID = %q, want %q", resp.VoiceID, "clone-leading-zero")
		}
	})

	t.Run("File.Upload and CloneVoice work together with numeric file_id", func(t *testing.T) {
		t.Parallel()

		var uploadCalls atomic.Int32
		var cloneCalls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case defaultFileUploadPath:
				uploadCalls.Add(1)
				if err := r.ParseMultipartForm(1 << 20); err != nil {
					t.Fatalf("ParseMultipartForm() error = %v", err)
				}

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"data":{"file_id":372103253696905,"file_name":"sample.wav","content_type":"audio/wav"}}`))
				return

			case defaultVoiceClonePath:
				cloneCalls.Add(1)
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("ReadAll(r.Body) error = %v", err)
				}

				var payload map[string]json.RawMessage
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("Unmarshal(request body) error = %v", err)
				}

				if got := strings.TrimSpace(string(payload["file_id"])); got != "372103253696905" {
					t.Fatalf("clone payload file_id token = %q, want %q", got, "372103253696905")
				}

				if got := strings.TrimSpace(string(payload["voice_id"])); got != `"clone-from-upload"` {
					t.Fatalf("clone payload voice_id token = %q, want %q", got, `"clone-from-upload"`)
				}

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"voice_id":"clone-from-upload"}`))
				return
			}

			http.NotFound(w, r)
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		uploadResp, err := client.File.Upload(context.Background(), FileUploadRequest{
			Purpose:     "voice_clone",
			FileName:    "sample.wav",
			ContentType: "audio/wav",
			Data:        []byte("audio-content"),
		})
		if err != nil {
			t.Fatalf("File.Upload() error = %v, want nil", err)
		}

		if uploadResp.FileID != "372103253696905" {
			t.Fatalf("uploadResp.FileID = %q, want %q", uploadResp.FileID, "372103253696905")
		}

		cloneResp, err := client.Voice.CloneVoice(context.Background(), &CloneVoiceRequest{
			VoiceID: "clone-from-upload",
			FileID:  uploadResp.FileID,
		})
		if err != nil {
			t.Fatalf("CloneVoice() error = %v, want nil", err)
		}

		if cloneResp.VoiceID != "clone-from-upload" {
			t.Fatalf("cloneResp.VoiceID = %q, want %q", cloneResp.VoiceID, "clone-from-upload")
		}

		if got := uploadCalls.Load(); got != 1 {
			t.Fatalf("uploadCalls = %d, want 1", got)
		}

		if got := cloneCalls.Load(); got != 1 {
			t.Fatalf("cloneCalls = %d, want 1", got)
		}
	})

	t.Run("missing voice_id fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		_, err = client.Voice.CloneVoice(context.Background(), &CloneVoiceRequest{AudioURL: "https://cdn.example.com/a.wav"})
		if err == nil {
			t.Fatal("CloneVoice() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "voice_id") {
			t.Fatalf("CloneVoice() error = %v, want voice_id validation", err)
		}
	})

	t.Run("missing input source fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		_, err = client.Voice.CloneVoice(context.Background(), &CloneVoiceRequest{VoiceID: "clone-id"})
		if err == nil {
			t.Fatal("CloneVoice() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "audio_url") || !strings.Contains(err.Error(), "file_id") {
			t.Fatalf("CloneVoice() error = %v, want source validation", err)
		}
	})

	t.Run("nil request fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		_, err = client.Voice.CloneVoice(context.Background(), nil)
		if err == nil {
			t.Fatal("CloneVoice() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "request is nil") {
			t.Fatalf("CloneVoice() error = %v, want request is nil", err)
		}
	})

	t.Run("http error returns unified APIError", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		_, err := client.Voice.CloneVoice(context.Background(), &CloneVoiceRequest{
			VoiceID:  "clone-id",
			AudioURL: "https://cdn.example.com/a.wav",
		})
		if err == nil {
			t.Fatal("CloneVoice() error = nil, want non-nil")
		}

		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("CloneVoice() error type = %T, want *protocol.APIError", err)
		}

		if apiErr.HTTPStatus != http.StatusUnauthorized {
			t.Fatalf("apiErr.HTTPStatus = %d, want %d", apiErr.HTTPStatus, http.StatusUnauthorized)
		}
	})

	t.Run("base_resp error returns unified APIError", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":2038,"status_msg":"no cloning permission"}}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		_, err := client.Voice.CloneVoice(context.Background(), &CloneVoiceRequest{
			VoiceID: "clone-id",
			FileID:  "file_123",
		})
		if err == nil {
			t.Fatal("CloneVoice() error = nil, want non-nil")
		}

		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("CloneVoice() error type = %T, want *protocol.APIError", err)
		}

		if apiErr.StatusCode != 2038 || apiErr.StatusMsg != "no cloning permission" {
			t.Fatalf("apiErr = %+v, want status_code=2038 status_msg=no cloning permission", apiErr)
		}
	})

	t.Run("timeout canceled by context", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(120 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"voice_id":"clone-id"}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		_, err := client.Voice.CloneVoice(ctx, &CloneVoiceRequest{
			VoiceID:  "clone-id",
			AudioURL: "https://cdn.example.com/a.wav",
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("CloneVoice() error = %v, want context deadline exceeded", err)
		}
	})

	t.Run("explicit context cancel", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = client.Voice.CloneVoice(ctx, &CloneVoiceRequest{
			VoiceID:  "clone-id",
			AudioURL: "https://cdn.example.com/a.wav",
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("CloneVoice() error = %v, want context canceled", err)
		}
	})
}

func TestVoiceDesignCloneValidation(t *testing.T) {
	t.Parallel()

	t.Run("nil service returns initialization error for design", func(t *testing.T) {
		t.Parallel()

		var service *VoiceService
		_, err := service.DesignVoice(context.Background(), &DesignVoiceRequest{
			Prompt:      "hello",
			PreviewText: "world",
		})
		if err == nil {
			t.Fatal("DesignVoice() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "not initialized") {
			t.Fatalf("DesignVoice() error = %v, want initialization error", err)
		}
	})

	t.Run("nil service returns initialization error for clone", func(t *testing.T) {
		t.Parallel()

		var service *VoiceService
		_, err := service.CloneVoice(context.Background(), &CloneVoiceRequest{
			VoiceID: "clone-id",
			FileID:  "file_1",
		})
		if err == nil {
			t.Fatal("CloneVoice() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "not initialized") {
			t.Fatalf("CloneVoice() error = %v, want initialization error", err)
		}
	})
}

func TestDeleteVoice(t *testing.T) {
	t.Parallel()

	t.Run("success preserves raw payload", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != defaultVoiceDeletePath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultVoiceDeletePath)
			}

			var payload deleteVoiceWireRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode(request body) error = %v", err)
			}
			if payload.VoiceID != "voice-to-delete" {
				t.Fatalf("payload.voice_id = %q, want voice-to-delete", payload.VoiceID)
			}
			if payload.VoiceType != VoiceDeleteTypeGeneration {
				t.Fatalf("payload.voice_type = %q, want %q", payload.VoiceType, VoiceDeleteTypeGeneration)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"voice_id":"voice-to-delete","request_id":"req-delete"}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.Voice.DeleteVoice(context.Background(), DeleteVoiceRequest{VoiceID: " voice-to-delete "})
		if err != nil {
			t.Fatalf("DeleteVoice() error = %v, want nil", err)
		}
		if resp.VoiceID != "voice-to-delete" {
			t.Fatalf("resp.VoiceID = %q, want voice-to-delete", resp.VoiceID)
		}
		if _, ok := resp.Raw["request_id"]; !ok {
			t.Fatalf("resp.Raw = %v, want request_id", resp.Raw)
		}
	})

	t.Run("empty voice id fails before request", func(t *testing.T) {
		t.Parallel()

		var called atomic.Bool
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			called.Store(true)
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		_, err := client.Voice.DeleteVoice(context.Background(), DeleteVoiceRequest{VoiceID: " \t "})
		if err == nil {
			t.Fatal("DeleteVoice() error = nil, want non-nil")
		}
		if called.Load() {
			t.Fatal("server called for invalid delete request, want local validation")
		}
	})

	t.Run("base_resp error returns unified APIError", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":2301,"status_msg":"voice not found"}}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		_, err := client.Voice.DeleteVoice(context.Background(), DeleteVoiceRequest{VoiceID: "missing"})
		if err == nil {
			t.Fatal("DeleteVoice() error = nil, want non-nil")
		}
		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("DeleteVoice() error type = %T, want *protocol.APIError", err)
		}
		if apiErr.StatusCode != 2301 {
			t.Fatalf("apiErr.StatusCode = %d, want 2301", apiErr.StatusCode)
		}
	})

	t.Run("http error returns unified APIError", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"bad gateway"}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		_, err := client.Voice.DeleteVoice(context.Background(), DeleteVoiceRequest{VoiceID: "voice-id"})
		if err == nil {
			t.Fatal("DeleteVoice() error = nil, want non-nil")
		}
		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("DeleteVoice() error type = %T, want *protocol.APIError", err)
		}
		if apiErr.HTTPStatus != http.StatusBadGateway {
			t.Fatalf("apiErr.HTTPStatus = %d, want %d", apiErr.HTTPStatus, http.StatusBadGateway)
		}
	})

	t.Run("explicit cloning voice type is forwarded", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload deleteVoiceWireRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode(request body) error = %v", err)
			}
			if payload.VoiceType != VoiceDeleteTypeCloning {
				t.Fatalf("payload.voice_type = %q, want %q", payload.VoiceType, VoiceDeleteTypeCloning)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"voice_id":"clone-voice"}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		_, err := client.Voice.DeleteVoice(context.Background(), DeleteVoiceRequest{
			VoiceID:   "clone-voice",
			VoiceType: VoiceDeleteTypeCloning,
		})
		if err != nil {
			t.Fatalf("DeleteVoice() error = %v, want nil", err)
		}
	})

	t.Run("invalid voice type fails before request", func(t *testing.T) {
		t.Parallel()

		var called atomic.Bool
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			called.Store(true)
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		_, err := client.Voice.DeleteVoice(context.Background(), DeleteVoiceRequest{
			VoiceID:   "voice-id",
			VoiceType: "system",
		})
		if err == nil {
			t.Fatal("DeleteVoice() error = nil, want invalid voice_type error")
		}
		if called.Load() {
			t.Fatal("server called for invalid voice_type, want local validation")
		}
	})
}

func TestVoiceUploadHelpers(t *testing.T) {
	t.Parallel()

	t.Run("clone audio helper sends official purpose", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != defaultFileUploadPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultFileUploadPath)
			}
			if got := r.FormValue("purpose"); got != VoiceUploadPurposeCloneAudio {
				t.Fatalf("purpose = %q, want %q", got, VoiceUploadPurposeCloneAudio)
			}
			file, header, err := r.FormFile(defaultFileFieldName)
			if err != nil {
				t.Fatalf("FormFile() error = %v", err)
			}
			defer file.Close()
			if header.Filename != "sample.mp3" {
				t.Fatalf("filename = %q, want sample.mp3", header.Filename)
			}
			body, err := io.ReadAll(file)
			if err != nil {
				t.Fatalf("ReadAll(file) error = %v", err)
			}
			if string(body) != "audio-bytes" {
				t.Fatalf("file body = %q, want audio-bytes", string(body))
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"file":{"file_id":123,"filename":"sample.mp3","purpose":"voice_clone","bytes":11}}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.Voice.UploadCloneAudio(context.Background(), UploadCloneAudioRequest{
			Filename:    "sample.mp3",
			Content:     strings.NewReader("audio-bytes"),
			ContentType: "audio/mpeg",
		})
		if err != nil {
			t.Fatalf("UploadCloneAudio() error = %v, want nil", err)
		}
		if resp.FileID != "123" {
			t.Fatalf("resp.FileID = %q, want 123", resp.FileID)
		}
	})

	t.Run("prompt audio helper sends official purpose", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.FormValue("purpose"); got != VoiceUploadPurposePromptAudio {
				t.Fatalf("purpose = %q, want %q", got, VoiceUploadPurposePromptAudio)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"file":{"file_id":"prompt-1","filename":"prompt.wav","purpose":"prompt_audio","bytes":4}}`))
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.Voice.UploadPromptAudio(context.Background(), UploadPromptAudioRequest{
			Filename: "prompt.wav",
			Content:  strings.NewReader("data"),
		})
		if err != nil {
			t.Fatalf("UploadPromptAudio() error = %v, want nil", err)
		}
		if resp.FileID != "prompt-1" {
			t.Fatalf("resp.FileID = %q, want prompt-1", resp.FileID)
		}
	})

	t.Run("nil content fails before request", func(t *testing.T) {
		t.Parallel()

		var called atomic.Bool
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			called.Store(true)
		}))
		defer srv.Close()

		client := newVoiceTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		_, err := client.Voice.UploadCloneAudio(context.Background(), UploadCloneAudioRequest{Filename: "sample.mp3"})
		if err == nil {
			t.Fatal("UploadCloneAudio() error = nil, want non-nil")
		}
		if called.Load() {
			t.Fatal("server called for nil upload content, want local validation")
		}
	})
}

func newVoiceTestClient(t *testing.T, srv *httptest.Server, retry transport.RetryConfig) *Client {
	t.Helper()

	client, err := NewClient(Config{
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
		Retry:      retry,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	return client
}
