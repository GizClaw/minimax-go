package minimax

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/GizClaw/minimax-go/internal/protocol"
	"github.com/GizClaw/minimax-go/internal/transport"
	"github.com/coder/websocket"
)

func TestSpeechWebSocket(t *testing.T) {
	t.Parallel()

	t.Run("handshake framing audio and terminal event", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != defaultSpeechWebSocketPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultSpeechWebSocketPath)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Fatalf("Authorization = %q, want Bearer test-token", got)
			}

			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				t.Fatalf("Accept() error = %v", err)
			}
			defer conn.Close(websocket.StatusNormalClosure, "")

			writeWebSocketJSON(t, conn, map[string]any{
				"event":      "connected_success",
				"session_id": "session-1",
				"trace_id":   "trace-1",
				"base_resp":  map[string]any{"status_code": 0, "status_msg": "success"},
			})

			start := readWebSocketJSON(t, conn)
			if start["event"] != "task_start" {
				t.Fatalf("start.event = %v, want task_start", start["event"])
			}
			if start["model"] != "speech-2.8-turbo" {
				t.Fatalf("start.model = %v, want speech-2.8-turbo", start["model"])
			}
			voiceSetting := start["voice_setting"].(map[string]any)
			if voiceSetting["voice_id"] != "voice-1" {
				t.Fatalf("voice_setting.voice_id = %v, want voice-1", voiceSetting["voice_id"])
			}
			audioSetting := start["audio_setting"].(map[string]any)
			if audioSetting["sample_rate"] != float64(32000) {
				t.Fatalf("audio_setting.sample_rate = %v, want 32000", audioSetting["sample_rate"])
			}

			writeWebSocketJSON(t, conn, map[string]any{
				"event":      "task_started",
				"session_id": "session-1",
				"trace_id":   "trace-2",
				"base_resp":  map[string]any{"status_code": 0, "status_msg": "success"},
			})

			continueMessage := readWebSocketJSON(t, conn)
			if continueMessage["event"] != "task_continue" || continueMessage["text"] != "hello" {
				t.Fatalf("continue message = %v, want task_continue hello", continueMessage)
			}
			finish := readWebSocketJSON(t, conn)
			if finish["event"] != "task_finish" {
				t.Fatalf("finish.event = %v, want task_finish", finish["event"])
			}

			writeWebSocketJSON(t, conn, map[string]any{
				"event":      "task_result",
				"session_id": "session-1",
				"trace_id":   "trace-3",
				"data":       map[string]any{"audio": "48656c6c6f"},
				"base_resp":  map[string]any{"status_code": 0, "status_msg": "success"},
			})
			writeWebSocketJSON(t, conn, map[string]any{
				"event":      "task_finished",
				"session_id": "session-1",
				"trace_id":   "trace-4",
				"base_resp":  map[string]any{"status_code": 0, "status_msg": "success"},
			})
		}))
		defer srv.Close()

		client := newSpeechWebSocketTestClient(t, srv, "test-token")
		sampleRate := 32000
		ws, err := client.Speech.OpenWebSocket(context.Background(), SpeechWebSocketRequest{
			Model:   "speech-2.8-turbo",
			Text:    "hello",
			VoiceID: "voice-1",
			AudioSetting: &SpeechAudioSetting{
				SampleRate: &sampleRate,
				Format:     "mp3",
			},
		})
		if err != nil {
			t.Fatalf("OpenWebSocket() error = %v, want nil", err)
		}
		defer ws.Close()

		event, err := ws.Next(context.Background())
		if err != nil {
			t.Fatalf("Next() audio error = %v, want nil", err)
		}
		if string(event.Audio) != "Hello" || event.RawHexAudio != "48656c6c6f" {
			t.Fatalf("event = %+v, want decoded Hello", event)
		}
		if event.Raw == nil {
			t.Fatal("event.Raw = nil, want raw frame")
		}

		done, err := ws.Next(context.Background())
		if err != nil {
			t.Fatalf("Next() done error = %v, want nil", err)
		}
		if !done.Done || done.Event != "task_finished" {
			t.Fatalf("done event = %+v, want task_finished done", done)
		}

		_, err = ws.Next(context.Background())
		if !errors.Is(err, io.EOF) {
			t.Fatalf("Next() after done error = %v, want io.EOF", err)
		}
	})

	t.Run("server error frame returns APIError", func(t *testing.T) {
		t.Parallel()

		srv := newSpeechWebSocketScriptServer(t, []any{
			map[string]any{"event": "connected_success", "base_resp": map[string]any{"status_code": 0, "status_msg": "success"}},
			map[string]any{"event": "task_started", "base_resp": map[string]any{"status_code": 0, "status_msg": "success"}},
			map[string]any{"event": "task_failed", "base_resp": map[string]any{"status_code": 1004, "status_msg": "bad text"}},
		})
		defer srv.Close()

		client := newSpeechWebSocketTestClient(t, srv, "")
		ws, err := client.Speech.OpenWebSocket(context.Background(), SpeechWebSocketRequest{
			Text:    "hello",
			VoiceID: "voice-1",
		})
		if err != nil {
			t.Fatalf("OpenWebSocket() error = %v, want nil", err)
		}
		defer ws.Close()

		_, err = ws.Next(context.Background())
		if err == nil {
			t.Fatal("Next() error = nil, want non-nil")
		}
		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("Next() error type = %T, want *protocol.APIError", err)
		}
		if apiErr.StatusCode != 1004 {
			t.Fatalf("apiErr.StatusCode = %d, want 1004", apiErr.StatusCode)
		}
	})

	t.Run("malformed frame returns decode error", func(t *testing.T) {
		t.Parallel()

		srv := newSpeechWebSocketScriptServer(t, []any{
			map[string]any{"event": "connected_success", "base_resp": map[string]any{"status_code": 0, "status_msg": "success"}},
			map[string]any{"event": "task_started", "base_resp": map[string]any{"status_code": 0, "status_msg": "success"}},
			rawWebSocketText("not-json"),
		})
		defer srv.Close()

		client := newSpeechWebSocketTestClient(t, srv, "")
		ws, err := client.Speech.OpenWebSocket(context.Background(), SpeechWebSocketRequest{
			Text:    "hello",
			VoiceID: "voice-1",
		})
		if err != nil {
			t.Fatalf("OpenWebSocket() error = %v, want nil", err)
		}
		defer ws.Close()

		_, err = ws.Next(context.Background())
		if err == nil {
			t.Fatal("Next() error = nil, want malformed frame error")
		}
	})

	t.Run("context cancellation interrupts read", func(t *testing.T) {
		t.Parallel()

		ready := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				t.Fatalf("Accept() error = %v", err)
			}
			defer conn.Close(websocket.StatusNormalClosure, "")

			writeWebSocketJSON(t, conn, map[string]any{"event": "connected_success", "base_resp": map[string]any{"status_code": 0, "status_msg": "success"}})
			_ = readWebSocketJSON(t, conn)
			writeWebSocketJSON(t, conn, map[string]any{"event": "task_started", "base_resp": map[string]any{"status_code": 0, "status_msg": "success"}})
			_ = readWebSocketJSON(t, conn)
			_ = readWebSocketJSON(t, conn)
			close(ready)
			time.Sleep(200 * time.Millisecond)
		}))
		defer srv.Close()

		client := newSpeechWebSocketTestClient(t, srv, "")
		ws, err := client.Speech.OpenWebSocket(context.Background(), SpeechWebSocketRequest{
			Text:    "hello",
			VoiceID: "voice-1",
		})
		if err != nil {
			t.Fatalf("OpenWebSocket() error = %v, want nil", err)
		}
		defer ws.Close()
		<-ready

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = ws.Next(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Next() error = %v, want context.Canceled", err)
		}
	})

	t.Run("close is idempotent", func(t *testing.T) {
		t.Parallel()

		srv := newSpeechWebSocketScriptServer(t, []any{
			map[string]any{"event": "connected_success", "base_resp": map[string]any{"status_code": 0, "status_msg": "success"}},
			map[string]any{"event": "task_started", "base_resp": map[string]any{"status_code": 0, "status_msg": "success"}},
		})
		defer srv.Close()

		client := newSpeechWebSocketTestClient(t, srv, "")
		ws, err := client.Speech.OpenWebSocket(context.Background(), SpeechWebSocketRequest{
			Text:    "hello",
			VoiceID: "voice-1",
		})
		if err != nil {
			t.Fatalf("OpenWebSocket() error = %v, want nil", err)
		}
		if err := ws.Close(); err != nil {
			t.Fatalf("Close() error = %v, want nil", err)
		}
		if err := ws.Close(); err != nil {
			t.Fatalf("second Close() error = %v, want nil", err)
		}
	})
}

func TestSpeechWebSocketValidation(t *testing.T) {
	t.Parallel()

	client, err := NewClient(Config{BaseURL: "https://api.minimax.io"})
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	_, err = client.Speech.OpenWebSocket(context.Background(), SpeechWebSocketRequest{Text: "hello"})
	if err == nil {
		t.Fatal("OpenWebSocket() error = nil, want voice_id validation error")
	}

	_, err = client.Speech.OpenWebSocket(context.Background(), SpeechWebSocketRequest{
		Text:         "hello",
		VoiceID:      "voice-1",
		OutputFormat: "url",
	})
	if err == nil {
		t.Fatal("OpenWebSocket() error = nil, want output format validation error")
	}
}

type rawWebSocketText string

func newSpeechWebSocketScriptServer(t *testing.T, messages []any) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Fatalf("Accept() error = %v", err)
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		for idx, message := range messages {
			if idx == 1 {
				_ = readWebSocketJSON(t, conn)
			}
			if idx == 2 {
				_ = readWebSocketJSON(t, conn)
				_ = readWebSocketJSON(t, conn)
			}
			switch value := message.(type) {
			case rawWebSocketText:
				if err := conn.Write(context.Background(), websocket.MessageText, []byte(value)); err != nil {
					t.Fatalf("Write(raw) error = %v", err)
				}
			default:
				writeWebSocketJSON(t, conn, value)
			}
		}
		time.Sleep(20 * time.Millisecond)
	}))
}

func newSpeechWebSocketTestClient(t *testing.T, srv *httptest.Server, apiKey string) *Client {
	t.Helper()

	client, err := NewClient(Config{
		BaseURL:    srv.URL,
		APIKey:     apiKey,
		HTTPClient: srv.Client(),
		Retry:      transport.RetryConfig{MaxAttempts: 1},
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}
	return client
}

func writeWebSocketJSON(t *testing.T, conn *websocket.Conn, payload any) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal(%T) error = %v", payload, err)
	}
	if err := conn.Write(context.Background(), websocket.MessageText, body); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
}

func readWebSocketJSON(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()

	messageType, body, err := conn.Read(context.Background())
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if messageType != websocket.MessageText {
		t.Fatalf("message type = %v, want text", messageType)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", string(body), err)
	}
	return payload
}
