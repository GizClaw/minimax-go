package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"github.com/coder/websocket"
)

func TestBidiChunkingPreservesUnicodeWhitespaceAndSize(t *testing.T) {
	text := " Hello 世界。他说：“价钱是1,000元。” "
	for _, size := range []int{1, 4, 10000} {
		chunks := bidiTextChunks(text, size)
		if strings.Join(chunks, "") != text {
			t.Fatalf("text lost at size=%d: %q", size, chunks)
		}
		for _, chunk := range chunks {
			if !utf8.ValidString(chunk) || utf8.RuneCountInString(chunk) > 10000 {
				t.Fatal("invalid wire chunk")
			}
		}
	}
	text = strings.Repeat(" ", 20000) + "Hello"
	if got := strings.Join(bidiTextChunks(text, 10000), ""); got != text {
		t.Fatal("long whitespace input lost text")
	}
}

func TestBidiHelpDoesNotPrintEnvironmentKey(t *testing.T) {
	t.Setenv("MINIMAX_API_KEY", "real-looking-private-key")
	var output bytes.Buffer
	if err := runBidiCommand([]string{"-h"}, io.Discard, &output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "real-looking-private-key") {
		t.Fatal("help printed environment credential")
	}
}

func TestComparisonUsesAllProtocolsAndReusesBidiSession(t *testing.T) {
	var bidiConnections atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/t2a_v2" {
			var request map[string]any
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
				return
			}
			options, _ := request["stream_options"].(map[string]any)
			if request["stream"] != true || options["exclude_aggregated_audio"] != true {
				t.Error("missing HTTP streaming options")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"data\":{\"audio\":\"00000100\",\"status\":1}}\n\ndata: {\"data\":{\"status\":2},\"extra_info\":{\"usage_characters\":7}}\n\n")
			return
		}
		if r.URL.Path == "/ws/v1/t2a_v2_bidi" {
			bidiConnections.Add(1)
		} else if r.URL.Path != "/ws/v1/t2a_v2" {
			http.NotFound(w, r)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		write := func(text string) bool {
			return conn.Write(r.Context(), websocket.MessageText, []byte(text)) == nil
		}
		if !write(`{"event":"connected_success"}`) {
			return
		}
		for {
			_, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			var request struct{ Event, Text string }
			if err := json.Unmarshal(data, &request); err != nil {
				t.Error(err)
				return
			}
			switch request.Event {
			case "task_start":
				if !write(`{"event":"task_started"}`) {
					return
				}
			case "task_continue":
				if !write(`{"event":"sentence_start"}`) ||
					!write(`{"event":"task_continued","data":{"audio":"00000100"},"is_final":true,"extra_info":{"usage_characters":7}}`) ||
					!write(`{"event":"sentence_end"}`) {
					return
				}
			case "task_flush":
				if !write(`{"event":"task_flushed"}`) {
					return
				}
			case "task_finish":
				write(`{"event":"task_finished"}`)
				return
			default:
				t.Errorf("unexpected event: %s", request.Event)
				return
			}
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	if err := runSpeechComparisonCommand([]string{
		"-api-key", "test", "-base-url", server.URL, "-model", "speech-2.6-turbo",
		"-voice-id", "voice", "-text", "Hello 世界", "-chunk-runes", "1", "-chunk-delay", "0s",
		"-iterations", "2", "-output", dir,
	}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "comparison.json"))
	if err != nil {
		t.Fatal(err)
	}
	var report speechComparison
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Runs) != 6 || bidiConnections.Load() != 1 {
		t.Fatalf("runs=%d bidi connections=%d", len(report.Runs), bidiConnections.Load())
	}
	for _, run := range report.Runs {
		if run.Timing.Status != "complete" || run.Timing.AudioBytes == 0 {
			t.Fatalf("incomplete run: %+v", run)
		}
		if run.Timing.Protocol == "bidi" && (!run.Timing.FlushAcknowledged || run.Timing.SentenceEnds == 0 || run.ConnectionReused != (run.Iteration > 1)) {
			t.Fatalf("bidi lifecycle lost: %+v", run)
		}
		if run.Timing.Protocol == "http" && run.ConnectionReused != (run.Iteration > 1) {
			t.Fatalf("HTTP transport not reused: %+v", run)
		}
	}
}
