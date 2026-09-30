package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestRunWebSocketReportsBilledCharacters(t *testing.T) {
	for _, test := range []struct {
		name       string
		finalFrame map[string]any
		wantUsage  string
	}{
		{
			name: "final frame reports usage",
			finalFrame: map[string]any{
				"event": "task_continued", "is_final": true,
				"data":       map[string]any{"audio": ""},
				"extra_info": map[string]any{"usage_characters": 26},
			},
			wantUsage: "billed usage characters: 26",
		},
		{
			name:       "usage absent",
			finalFrame: map[string]any{"event": "task_continued", "is_final": true, "data": map[string]any{"audio": ""}},
			wantUsage:  "billed usage characters: not reported",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			srv := newSpeechWebSocketExampleServer(t, test.finalFrame)
			defer srv.Close()
			output := filepath.Join(t.TempDir(), "speech.mp3")
			var stdout bytes.Buffer
			err := runWebSocket(webSocketOptions{
				apiKey: "test-key", baseURL: srv.URL, text: "你好，世界！Hello world 123.",
				model: "speech-2.8-turbo", voiceID: "voice-1", timeout: 5 * time.Second, output: output,
			}, &stdout)
			if err != nil {
				t.Fatalf("runWebSocket() error = %v", err)
			}
			audio, err := os.ReadFile(output)
			if err != nil || string(audio) != "Hello" {
				t.Fatalf("written audio = %q, %v; want Hello", audio, err)
			}
			if !strings.Contains(stdout.String(), test.wantUsage) {
				t.Fatalf("stdout = %q, want %q", stdout.String(), test.wantUsage)
			}
		})
	}
}

// newSpeechWebSocketExampleServer serves one T2A task: one audio frame, the
// given final frame, and task_finished.
func newSpeechWebSocketExampleServer(t *testing.T, finalFrame map[string]any) *httptest.Server {
	t.Helper()
	ok := map[string]any{"status_code": 0, "status_msg": "success"}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept() error = %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		ctx := context.Background()
		write := func(payload map[string]any) {
			payload["base_resp"] = ok
			data, _ := json.Marshal(payload)
			if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
				t.Errorf("write %v: %v", payload["event"], err)
			}
		}
		read := func() {
			if _, _, err := conn.Read(ctx); err != nil {
				t.Errorf("read client message: %v", err)
			}
		}
		write(map[string]any{"event": "connected_success"})
		read() // task_start
		write(map[string]any{"event": "task_started"})
		read() // task_continue
		read() // task_finish
		write(map[string]any{"event": "task_continued", "is_final": false, "data": map[string]any{"audio": "48656c6c6f"}})
		write(finalFrame)
		write(map[string]any{"event": "task_finished"})
	}))
}
