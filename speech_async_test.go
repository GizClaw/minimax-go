package minimax

import (
	"context"
	"encoding/json"
	"errors"
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

func TestSpeechAsync(t *testing.T) {
	t.Parallel()

	t.Run("speech service delegates async submit and query", func(t *testing.T) {
		t.Parallel()

		var queryAttempts atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case defaultSpeechAsyncSubmitPath:
				if r.Method != http.MethodPost {
					t.Fatalf("submit method = %s, want POST", r.Method)
				}

				var payload struct {
					Model        string `json:"model"`
					Text         string `json:"text"`
					VoiceSetting struct {
						VoiceID string   `json:"voice_id"`
						Vol     *float64 `json:"vol"`
					} `json:"voice_setting"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatalf("Decode(submit request body) error = %v", err)
				}

				if payload.Model != defaultSpeechModel {
					t.Fatalf("payload.Model = %q, want %q", payload.Model, defaultSpeechModel)
				}

				if payload.Text != "hello async" {
					t.Fatalf("payload.Text = %q, want hello async", payload.Text)
				}

				if payload.VoiceSetting.VoiceID != "English_Graceful_Lady" {
					t.Fatalf("payload.voice_setting.voice_id = %q, want English_Graceful_Lady", payload.VoiceSetting.VoiceID)
				}

				if payload.VoiceSetting.Vol == nil || *payload.VoiceSetting.Vol != 0 {
					t.Fatalf("payload.voice_setting.vol = %v, want 0", payload.VoiceSetting.Vol)
				}

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"task_id":"task_123","status":"queued"}`))

			case defaultSpeechAsyncQueryPath:
				if r.Method != http.MethodGet {
					t.Fatalf("query method = %s, want GET", r.Method)
				}

				if got := r.URL.Query().Get("task_id"); got != "task_123" {
					t.Fatalf("query.task_id = %q, want task_123", got)
				}

				attempt := queryAttempts.Add(1)
				w.Header().Set("Content-Type", "application/json")
				if attempt == 1 {
					_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"task_id":"task_123","status":"Processing"}`))
					return
				}

				_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"task_id":"task_123","status":"Success","result":{"audio_url":"https://cdn.example.com/audio.mp3","duration":12.5,"size":2048,"format":"mp3"}}`))

			default:
				t.Fatalf("unexpected path %s", r.URL.Path)
			}
		}))
		defer srv.Close()

		client := newSpeechAsyncTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})

		zero := 0.0
		submitResp, err := client.Speech.SubmitAsync(context.Background(), SpeechAsyncSubmitRequest{
			Text:    " hello async ",
			VoiceID: "English_Graceful_Lady",
			Vol:     &zero,
		})
		if err != nil {
			t.Fatalf("Speech.SubmitAsync() error = %v, want nil", err)
		}

		if submitResp.TaskID != "task_123" {
			t.Fatalf("submitResp.TaskID = %q, want task_123", submitResp.TaskID)
		}

		if submitResp.Status != SpeechTaskStateQueued {
			t.Fatalf("submitResp.Status = %q, want queued", submitResp.Status)
		}

		runningResp, err := client.Speech.GetAsyncTask(context.Background(), "task_123")
		if err != nil {
			t.Fatalf("Speech.GetAsyncTask(running) error = %v, want nil", err)
		}

		if runningResp.Status != SpeechTaskStateRunning {
			t.Fatalf("runningResp.Status = %q, want running", runningResp.Status)
		}

		succeededResp, err := client.Speech.GetAsyncTask(context.Background(), "task_123")
		if err != nil {
			t.Fatalf("Speech.GetAsyncTask(succeeded) error = %v, want nil", err)
		}

		if succeededResp.Status != SpeechTaskStateSucceeded {
			t.Fatalf("succeededResp.Status = %q, want succeeded", succeededResp.Status)
		}

		if succeededResp.Result.AudioURL != "https://cdn.example.com/audio.mp3" {
			t.Fatalf("succeededResp.Result.AudioURL = %q, want https://cdn.example.com/audio.mp3", succeededResp.Result.AudioURL)
		}
	})
}

func TestSubmitAsync(t *testing.T) {
	t.Parallel()

	t.Run("success returns task id and normalized status", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != defaultSpeechAsyncSubmitPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultSpeechAsyncSubmitPath)
			}

			var payload struct {
				Model      string `json:"model"`
				Text       string `json:"text"`
				TextFileID string `json:"text_file_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode(request body) error = %v", err)
			}

			if payload.Model != defaultSpeechModel {
				t.Fatalf("payload.Model = %q, want %q", payload.Model, defaultSpeechModel)
			}

			if payload.Text != "hello world" {
				t.Fatalf("payload.Text = %q, want hello world", payload.Text)
			}

			if payload.TextFileID != "" {
				t.Fatalf("payload.TextFileID = %q, want empty", payload.TextFileID)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"data":{"task_id":"task-001","status":"processing","file_id":12345,"usage_characters":11},"request_id":"req-async-1"}`))
		}))
		defer srv.Close()

		client := newSpeechAsyncTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.SpeechAsync.SubmitAsync(context.Background(), SpeechAsyncSubmitRequest{Text: " hello world "})
		if err != nil {
			t.Fatalf("SubmitAsync() error = %v, want nil", err)
		}

		if resp.TaskID != "task-001" {
			t.Fatalf("resp.TaskID = %q, want task-001", resp.TaskID)
		}

		if resp.Status != SpeechTaskStateRunning {
			t.Fatalf("resp.Status = %q, want running", resp.Status)
		}

		if resp.FileID != "12345" {
			t.Fatalf("resp.FileID = %q, want 12345", resp.FileID)
		}

		if resp.UsageCharacters == nil || *resp.UsageCharacters != 11 {
			t.Fatalf("resp.UsageCharacters = %v, want 11", resp.UsageCharacters)
		}

		if _, ok := resp.Raw["request_id"]; !ok {
			t.Fatalf("resp.Raw = %v, want request_id field", resp.Raw)
		}

		if _, ok := resp.Raw["data"]; !ok {
			t.Fatalf("resp.Raw = %v, want raw data payload", resp.Raw)
		}
	})

	t.Run("base_resp error returns unified APIError", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":2301,"status_msg":"invalid voice"}}`))
		}))
		defer srv.Close()

		client := newSpeechAsyncTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		_, err := client.SpeechAsync.SubmitAsync(context.Background(), SpeechAsyncSubmitRequest{Text: "hello"})
		if err == nil {
			t.Fatal("SubmitAsync() error = nil, want non-nil")
		}

		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("SubmitAsync() error type = %T, want *protocol.APIError", err)
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
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"task_id":"task_timeout"}`))
		}))
		defer srv.Close()

		client := newSpeechAsyncTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		_, err := client.SpeechAsync.SubmitAsync(ctx, SpeechAsyncSubmitRequest{Text: "hello"})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("SubmitAsync() error = %v, want context deadline exceeded", err)
		}
	})

	t.Run("explicit context cancel", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v, want nil", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = client.SpeechAsync.SubmitAsync(ctx, SpeechAsyncSubmitRequest{Text: "hello"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("SubmitAsync() error = %v, want context canceled", err)
		}
	})

	t.Run("empty text and text_file_id fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v, want nil", err)
		}

		_, err = client.SpeechAsync.SubmitAsync(context.Background(), SpeechAsyncSubmitRequest{})
		if err == nil {
			t.Fatal("SubmitAsync() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "text and text_file_id") {
			t.Fatalf("SubmitAsync() error = %v, want text/text_file_id validation", err)
		}
	})
}

func TestGetAsyncTask(t *testing.T) {
	t.Parallel()

	t.Run("running and succeeded states are normalized", func(t *testing.T) {
		t.Parallel()

		var attempts atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != defaultSpeechAsyncQueryPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultSpeechAsyncQueryPath)
			}

			if got := r.URL.Query().Get("task_id"); got != "task-123" {
				t.Fatalf("query.task_id = %q, want task-123", got)
			}

			w.Header().Set("Content-Type", "application/json")
			if attempts.Add(1) == 1 {
				_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"task":{"task_id":"task-123","status":"Processing"}}`))
				return
			}

			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"task":{"task_id":"task-123","status":"Success"},"result":{"file_id":"file-123","audio_url":"https://cdn.example.com/task-123.mp3","duration":5.5,"size":1024,"format":"mp3","sample_rate":32000,"bitrate":128000,"channel":1}}`))
		}))
		defer srv.Close()

		client := newSpeechAsyncTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})

		runningResp, err := client.SpeechAsync.GetAsyncTask(context.Background(), "task-123")
		if err != nil {
			t.Fatalf("GetAsyncTask(running) error = %v, want nil", err)
		}

		if runningResp.Status != SpeechTaskStateRunning {
			t.Fatalf("runningResp.Status = %q, want running", runningResp.Status)
		}

		succeededResp, err := client.SpeechAsync.GetAsyncTask(context.Background(), "task-123")
		if err != nil {
			t.Fatalf("GetAsyncTask(succeeded) error = %v, want nil", err)
		}

		if succeededResp.Status != SpeechTaskStateSucceeded {
			t.Fatalf("succeededResp.Status = %q, want succeeded", succeededResp.Status)
		}

		if succeededResp.Result.FileID != "file-123" {
			t.Fatalf("succeededResp.Result.FileID = %q, want file-123", succeededResp.Result.FileID)
		}

		if succeededResp.Result.AudioURL != "https://cdn.example.com/task-123.mp3" {
			t.Fatalf("succeededResp.Result.AudioURL = %q, want https://cdn.example.com/task-123.mp3", succeededResp.Result.AudioURL)
		}

		if succeededResp.Result.Meta.DurationSeconds == nil || *succeededResp.Result.Meta.DurationSeconds != 5.5 {
			t.Fatalf("succeededResp.Result.Meta.DurationSeconds = %v, want 5.5", succeededResp.Result.Meta.DurationSeconds)
		}

		if succeededResp.Result.Meta.SizeBytes == nil || *succeededResp.Result.Meta.SizeBytes != 1024 {
			t.Fatalf("succeededResp.Result.Meta.SizeBytes = %v, want 1024", succeededResp.Result.Meta.SizeBytes)
		}

		if succeededResp.Result.Meta.Format != "mp3" {
			t.Fatalf("succeededResp.Result.Meta.Format = %q, want mp3", succeededResp.Result.Meta.Format)
		}

		if succeededResp.Result.Meta.SampleRate == nil || *succeededResp.Result.Meta.SampleRate != 32000 {
			t.Fatalf("succeededResp.Result.Meta.SampleRate = %v, want 32000", succeededResp.Result.Meta.SampleRate)
		}
	})

	t.Run("succeeded with url and no audio bytes is valid", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"task_id":"task-url-only","status":"completed","result":{"audio_url":"https://cdn.example.com/url-only.mp3","duration":1.2,"size":256}}`))
		}))
		defer srv.Close()

		client := newSpeechAsyncTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.SpeechAsync.GetAsyncTask(context.Background(), "task-url-only")
		if err != nil {
			t.Fatalf("GetAsyncTask() error = %v, want nil", err)
		}

		if resp.Status != SpeechTaskStateSucceeded {
			t.Fatalf("resp.Status = %q, want succeeded", resp.Status)
		}

		if resp.Result.AudioURL != "https://cdn.example.com/url-only.mp3" {
			t.Fatalf("resp.Result.AudioURL = %q, want https://cdn.example.com/url-only.mp3", resp.Result.AudioURL)
		}

		if len(resp.Result.Audio) != 0 {
			t.Fatalf("len(resp.Result.Audio) = %d, want 0", len(resp.Result.Audio))
		}

		if _, ok := resp.Raw["result"]; !ok {
			t.Fatalf("resp.Raw = %v, want raw result payload", resp.Raw)
		}
	})

	t.Run("failed task state maps to failed and extracts error_msg", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"task_id":"task-failed","status":"failed","error_msg":"quota exceeded"}`))
		}))
		defer srv.Close()

		client := newSpeechAsyncTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.SpeechAsync.GetAsyncTask(context.Background(), "task-failed")
		if err != nil {
			t.Fatalf("GetAsyncTask() error = %v, want nil", err)
		}

		if resp.Status != SpeechTaskStateFailed {
			t.Fatalf("resp.Status = %q, want failed", resp.Status)
		}

		if resp.ErrorMessage != "quota exceeded" {
			t.Fatalf("resp.ErrorMessage = %q, want quota exceeded", resp.ErrorMessage)
		}
	})

	t.Run("canceled task state maps to failed and extracts nested message", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"task":{"task_id":"task-canceled","status":"canceled","message":"manually canceled"}}`))
		}))
		defer srv.Close()

		client := newSpeechAsyncTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.SpeechAsync.GetAsyncTask(context.Background(), "task-canceled")
		if err != nil {
			t.Fatalf("GetAsyncTask() error = %v, want nil", err)
		}

		if resp.Status != SpeechTaskStateFailed {
			t.Fatalf("resp.Status = %q, want failed", resp.Status)
		}

		if resp.ErrorMessage != "manually canceled" {
			t.Fatalf("resp.ErrorMessage = %q, want manually canceled", resp.ErrorMessage)
		}
	})

	t.Run("failed task without error fields falls back to raw status", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"task_id":"task-fallback","status":"Failed"}`))
		}))
		defer srv.Close()

		client := newSpeechAsyncTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.SpeechAsync.GetAsyncTask(context.Background(), "task-fallback")
		if err != nil {
			t.Fatalf("GetAsyncTask() error = %v, want nil", err)
		}

		if resp.Status != SpeechTaskStateFailed {
			t.Fatalf("resp.Status = %q, want failed", resp.Status)
		}

		if resp.ErrorMessage != "Failed" {
			t.Fatalf("resp.ErrorMessage = %q, want Failed", resp.ErrorMessage)
		}
	})

	t.Run("succeeded with hex payload decodes audio", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"status":"success","data":{"audio_hex":"48656c6c6f"}}`))
		}))
		defer srv.Close()

		client := newSpeechAsyncTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		resp, err := client.SpeechAsync.GetAsyncTask(context.Background(), "task-hex")
		if err != nil {
			t.Fatalf("GetAsyncTask() error = %v, want nil", err)
		}

		if got := string(resp.Result.Audio); got != "Hello" {
			t.Fatalf("resp.Result.Audio = %q, want Hello", got)
		}
	})

	t.Run("base_resp non-zero returns unified APIError", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":2021,"status_msg":"task not found"}}`))
		}))
		defer srv.Close()

		client := newSpeechAsyncTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		_, err := client.SpeechAsync.GetAsyncTask(context.Background(), "task-not-found")
		if err == nil {
			t.Fatal("GetAsyncTask() error = nil, want non-nil")
		}

		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("GetAsyncTask() error type = %T, want *protocol.APIError", err)
		}

		if apiErr.StatusCode != 2021 || apiErr.StatusMsg != "task not found" {
			t.Fatalf("apiErr = %+v, want status_code=2021 status_msg=task not found", apiErr)
		}
	})

	t.Run("http 5xx returns unified APIError", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"upstream failed"}`))
		}))
		defer srv.Close()

		client := newSpeechAsyncTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		_, err := client.SpeechAsync.GetAsyncTask(context.Background(), "task-500")
		if err == nil {
			t.Fatal("GetAsyncTask() error = nil, want non-nil")
		}

		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("GetAsyncTask() error type = %T, want *protocol.APIError", err)
		}

		if apiErr.HTTPStatus != http.StatusBadGateway {
			t.Fatalf("apiErr.HTTPStatus = %d, want %d", apiErr.HTTPStatus, http.StatusBadGateway)
		}
	})

	t.Run("invalid hex returns decode error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"status":"succeeded","audio_hex":"xyz"}`))
		}))
		defer srv.Close()

		client := newSpeechAsyncTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		_, err := client.SpeechAsync.GetAsyncTask(context.Background(), "task-invalid-hex")
		if !errors.Is(err, codec.ErrInvalidHexAudio) {
			t.Fatalf("GetAsyncTask() error = %v, want codec.ErrInvalidHexAudio", err)
		}
	})

	t.Run("timeout canceled by context", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(120 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"task_id":"task-timeout","status":"processing"}`))
		}))
		defer srv.Close()

		client := newSpeechAsyncTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		_, err := client.SpeechAsync.GetAsyncTask(ctx, "task-timeout")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("GetAsyncTask() error = %v, want context deadline exceeded", err)
		}
	})

	t.Run("explicit context cancel", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v, want nil", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = client.SpeechAsync.GetAsyncTask(ctx, "task-cancel")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("GetAsyncTask() error = %v, want context canceled", err)
		}
	})

	t.Run("empty task_id fails fast", func(t *testing.T) {
		t.Parallel()

		client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
		if err != nil {
			t.Fatalf("NewClient() error = %v, want nil", err)
		}

		_, err = client.SpeechAsync.GetAsyncTask(context.Background(), "  ")
		if err == nil {
			t.Fatal("GetAsyncTask() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "task_id") {
			t.Fatalf("GetAsyncTask() error = %v, want task_id validation", err)
		}
	})
}

func TestSpeechTask(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		input  string
		wanted SpeechTaskState
	}{
		{name: "queued alias", input: "pending", wanted: SpeechTaskStateQueued},
		{name: "running alias", input: "Processing", wanted: SpeechTaskStateRunning},
		{name: "succeeded alias", input: "TASK_STATUS_SUCCEED", wanted: SpeechTaskStateSucceeded},
		{name: "failed alias", input: "expired", wanted: SpeechTaskStateFailed},
		{name: "unknown state", input: "something-new", wanted: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := normalizeSpeechTaskState(tc.input)
			if got != tc.wanted {
				t.Fatalf("normalizeSpeechTaskState(%q) = %q, want %q", tc.input, got, tc.wanted)
			}
		})
	}

	if !SpeechTaskStateSucceeded.IsTerminal() {
		t.Fatal("SpeechTaskStateSucceeded.IsTerminal() = false, want true")
	}

	if !SpeechTaskStateFailed.IsTerminal() {
		t.Fatal("SpeechTaskStateFailed.IsTerminal() = false, want true")
	}

	if SpeechTaskStateRunning.IsTerminal() {
		t.Fatal("SpeechTaskStateRunning.IsTerminal() = true, want false")
	}
}

func TestSpeechAsyncSubmitOfficialFields(t *testing.T) {
	t.Parallel()

	sampleRate := 32000
	bitrate := 128000
	channel := 2
	enabled := true

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode(request body) error = %v", err)
		}

		if payload["text_file_id"] != float64(12345) {
			t.Fatalf("payload.text_file_id = %v, want numeric 12345", payload["text_file_id"])
		}
		if payload["output_format"] != "url" {
			t.Fatalf("payload.output_format = %v, want url", payload["output_format"])
		}
		if payload["language_boost"] != "English" {
			t.Fatalf("payload.language_boost = %v, want English", payload["language_boost"])
		}

		audioSetting := payload["audio_setting"].(map[string]any)
		if audioSetting["audio_sample_rate"] != float64(sampleRate) {
			t.Fatalf("audio_setting.audio_sample_rate = %v, want %d", audioSetting["audio_sample_rate"], sampleRate)
		}
		if audioSetting["sample_rate"] != nil {
			t.Fatalf("audio_setting.sample_rate = %v, want omitted", audioSetting["sample_rate"])
		}
		if audioSetting["bitrate"] != float64(bitrate) || audioSetting["channel"] != float64(channel) {
			t.Fatalf("audio_setting = %v, want bitrate/channel", audioSetting)
		}

		voiceSetting := payload["voice_setting"].(map[string]any)
		if voiceSetting["latex_read"] != true {
			t.Fatalf("voice_setting.latex_read = %v, want true", voiceSetting["latex_read"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"task_id":95157322514444,"file_id":95157322514496,"usage_characters":101}`))
	}))
	defer srv.Close()

	client := newSpeechAsyncTestClient(t, srv, transport.RetryConfig{MaxAttempts: 1})
	resp, err := client.SpeechAsync.SubmitAsync(context.Background(), SpeechAsyncSubmitRequest{
		Model:        "speech-2.8-hd",
		TextFileID:   "12345",
		VoiceID:      "voice-1",
		LatexRead:    &enabled,
		OutputFormat: "url",
		AudioSetting: &SpeechAudioSetting{
			SampleRate: &sampleRate,
			Bitrate:    &bitrate,
			Format:     "mp3",
			Channel:    &channel,
		},
		PronunciationDict: &SpeechPronunciationDict{Tone: []string{"dangerous/危险"}},
		LanguageBoost:     "English",
		SubtitleEnable:    &enabled,
		SubtitleType:      "sentence",
	})
	if err != nil {
		t.Fatalf("SubmitAsync() error = %v, want nil", err)
	}
	if resp.TaskID != "95157322514444" || resp.FileID != "95157322514496" {
		t.Fatalf("resp = %+v, want task/file IDs", resp)
	}
}

func newSpeechAsyncTestClient(t *testing.T, srv *httptest.Server, retry transport.RetryConfig) *Client {
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
