package minimax

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func newBidiTestServer(t *testing.T, tls bool, run func(context.Context, *websocket.Conn)) *httptest.Server {
	t.Helper()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != defaultSpeechBidiPath || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected bidi request: %s", r.URL.Path)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if !bidiTestWrite(t, ctx, conn, `{"event":"connected_success","connect_id":"connection"}`) {
			return
		}
		var start map[string]any
		if !bidiTestRead(t, ctx, conn, &start) {
			return
		}
		voice, _ := start["voice_setting"].(map[string]any)
		if start["event"] != "task_start" || start["model"] != "speech-2.6-turbo" || voice["voice_id"] != "voice" {
			t.Errorf("start configuration: %v", start)
			return
		}
		if !bidiTestWrite(t, ctx, conn, `{"event":"task_started","connect_id":"connection"}`) {
			return
		}
		run(ctx, conn)
	})
	var server *httptest.Server
	if tls {
		server = httptest.NewTLSServer(handler)
	} else {
		server = httptest.NewServer(handler)
	}
	t.Cleanup(server.Close)
	return server
}

func bidiTestWrite(t *testing.T, ctx context.Context, conn *websocket.Conn, text string) bool {
	t.Helper()
	if err := conn.Write(ctx, websocket.MessageText, []byte(text)); err != nil {
		t.Error(err)
		return false
	}
	return true
}

func bidiTestRead(t *testing.T, ctx context.Context, conn *websocket.Conn, value any) bool {
	t.Helper()
	_, data, err := conn.Read(ctx)
	if err == nil {
		err = json.Unmarshal(data, value)
	}
	if err != nil {
		t.Error(err)
	}
	return err == nil
}

func openBidiTestClient(t *testing.T, server *httptest.Server) *SpeechBidi {
	t.Helper()
	client, err := NewClient(Config{BaseURL: server.URL, APIKey: "test-key", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	bidi, err := client.Speech.OpenBidi(t.Context(), SpeechBidiRequest{Model: "speech-2.6-turbo", VoiceID: "voice", SessionID: "turn"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bidi.Close() })
	return bidi
}

func TestSpeechBidiLifecycleAndConfiguredTLSTransport(t *testing.T) {
	texts := make(chan string, 4)
	server := newBidiTestServer(t, true, func(ctx context.Context, conn *websocket.Conn) {
		for {
			var event struct{ Event, Text string }
			if !bidiTestRead(t, ctx, conn, &event) {
				return
			}
			switch event.Event {
			case "task_continue":
				texts <- event.Text
				for _, text := range []string{
					`{"event":"sentence_start","session_id":"turn","connect_id":"connection"}`,
					`{"event":"task_continued","data":{"audio":"0102"},"is_final":true,"extra_info":{"usage_characters":7}}`,
					`{"event":"sentence_end"}`,
				} {
					if !bidiTestWrite(t, ctx, conn, text) {
						return
					}
				}
			case "task_flush":
				if !bidiTestWrite(t, ctx, conn, `{"event":"task_flushed"}`) {
					return
				}
			case "task_cancel":
				if !bidiTestWrite(t, ctx, conn, `{"event":"task_canceled"}`) {
					return
				}
			case "task_finish":
				bidiTestWrite(t, ctx, conn, `{"event":"task_finished"}`)
				return
			default:
				t.Errorf("unexpected event: %s", event.Event)
				return
			}
		}
	})
	bidi := openBidiTestClient(t, server)
	for _, text := range []string{"他说：“价", "格是1,000元。”", " Hello, 世界!"} {
		if err := bidi.SendText(t.Context(), text); err != nil {
			t.Fatal(err)
		}
		for _, expected := range []string{"sentence_start", "task_continued", "sentence_end"} {
			event, err := bidi.Next(t.Context())
			if err != nil || event.Event != expected || event.Done {
				t.Fatalf("event=%+v err=%v want=%s", event, err, expected)
			}
			if expected == "task_continued" && (!event.IsFinal || string(event.Audio) != "\x01\x02" || event.UsageCharacters == nil || *event.UsageCharacters != 7) {
				t.Fatalf("audio/final/usage lost: %+v", event)
			}
			if expected == "sentence_start" && event.ConnectID != "connection" {
				t.Fatalf("connect ID lost: %+v", event)
			}
		}
		if got := <-texts; got != text {
			t.Fatalf("text changed: %q want %q", got, text)
		}
	}
	for _, control := range []struct {
		send func(context.Context) error
		want string
	}{{bidi.Flush, "task_flushed"}, {bidi.Cancel, "task_canceled"}} {
		if err := control.send(t.Context()); err != nil {
			t.Fatal(err)
		}
		event, err := bidi.Next(t.Context())
		if err != nil || event.Event != control.want || event.Done {
			t.Fatalf("control=%+v err=%v", event, err)
		}
	}
	if err := bidi.SendText(t.Context(), "打断后继续"); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := bidi.Next(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if err := bidi.Finish(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := bidi.Finish(t.Context()); err != nil {
		t.Fatal(err)
	}
	event, err := bidi.Next(t.Context())
	if err != nil || !event.Done || event.Event != "task_finished" {
		t.Fatalf("terminal=%+v err=%v", event, err)
	}
	if _, err := bidi.Next(t.Context()); !errors.Is(err, io.EOF) {
		t.Fatalf("after finish: %v", err)
	}
	if err := bidi.SendText(t.Context(), "late"); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("accepted input after finish: %v", err)
	}
}

func TestSpeechBidiSoftErrorCanRecover(t *testing.T) {
	server := newBidiTestServer(t, false, func(ctx context.Context, conn *websocket.Conn) {
		var value map[string]any
		if !bidiTestRead(t, ctx, conn, &value) || !bidiTestWrite(t, ctx, conn, `{"event":"task_failed","base_resp":{"status_code":2205,"status_msg":"queue full"}}`) {
			return
		}
		if !bidiTestRead(t, ctx, conn, &value) {
			return
		}
		bidiTestWrite(t, ctx, conn, `{"event":"task_continued","data":{"audio":"0102"}}`)
		if !bidiTestRead(t, ctx, conn, &value) {
			return
		}
		bidiTestWrite(t, ctx, conn, `{"event":"task_finished"}`)
	})
	bidi := openBidiTestClient(t, server)
	if err := bidi.SendText(t.Context(), "retry me"); err != nil {
		t.Fatal(err)
	}
	if _, err := bidi.Next(t.Context()); err == nil {
		t.Fatal("missing provider queue error")
	}
	if err := bidi.SendText(t.Context(), "retry me"); err != nil {
		t.Fatal(err)
	}
	event, err := bidi.Next(t.Context())
	if err != nil || len(event.Audio) != 2 {
		t.Fatalf("session did not recover: %+v %v", event, err)
	}
	if err := bidi.Finish(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := bidi.Next(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestSpeechBidiRejectsTruncationAndMalformedEvents(t *testing.T) {
	for _, payload := range []string{"", "{", `{"event":"task_continued","data":{"audio":"xyz"}}`, `{"event":"task_failed","base_resp":{"status_code":1004,"status_msg":"invalid model"}}`} {
		t.Run(fmt.Sprintf("%q", payload), func(t *testing.T) {
			server := newBidiTestServer(t, false, func(ctx context.Context, conn *websocket.Conn) {
				if payload == "" {
					_ = conn.Close(websocket.StatusNormalClosure, "")
				} else {
					bidiTestWrite(t, ctx, conn, payload)
				}
			})
			bidi := openBidiTestClient(t, server)
			_, err := bidi.Next(t.Context())
			if err == nil || payload == "" && !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("unexpected event accepted: %v", err)
			}
		})
	}
}

func TestSpeechBidiValidationCloseAndQueuedWriteCancellation(t *testing.T) {
	server := newBidiTestServer(t, false, func(ctx context.Context, conn *websocket.Conn) {
		_, _, _ = conn.Read(ctx)
	})
	bidi := openBidiTestClient(t, server)
	for _, text := range []string{string([]byte{0xff}), strings.Repeat("中", 10001)} {
		if err := bidi.SendText(t.Context(), text); err == nil {
			t.Fatal("invalid input reached network")
		}
	}
	<-bidi.writeGate
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := bidi.SendText(ctx, "queued"); !errors.Is(err, context.Canceled) {
		t.Fatalf("queued write ignored context: %v", err)
	}
	bidi.writeGate <- struct{}{}
	readDone := make(chan error, 1)
	go func() { _, err := bidi.Next(t.Context()); readDone <- err }()
	if err := bidi.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-readDone:
		if err == nil {
			t.Fatal("closed reader succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not release Next")
	}
}

func TestSpeechBidiReadsLargeProviderAudioFrames(t *testing.T) {
	server := newBidiTestServer(t, false, func(ctx context.Context, conn *websocket.Conn) {
		if !bidiTestWrite(t, ctx, conn, fmt.Sprintf(`{"event":"task_continued","data":{"audio":"%s"}}`, strings.Repeat("01", 40000))) {
			return
		}
		var value map[string]any
		if bidiTestRead(t, ctx, conn, &value) {
			bidiTestWrite(t, ctx, conn, `{"event":"task_finished"}`)
		}
	})
	bidi := openBidiTestClient(t, server)
	event, err := bidi.Next(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(event.Audio) != 40000 {
		t.Fatalf("large frame: audio=%d", len(event.Audio))
	}
	if err := bidi.Finish(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := bidi.Next(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestSpeechBidiStartupHonorsContextDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		// No startup acknowledgement: client deadline must release the read.
		_, _, _ = conn.Read(r.Context())
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	_, err = client.Speech.OpenBidi(ctx, SpeechBidiRequest{Model: "speech-2.6-turbo", VoiceID: "voice"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("startup deadline lost: %v", err)
	}
}
