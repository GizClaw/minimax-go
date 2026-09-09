package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	minimax "github.com/GizClaw/minimax-go"
)

func TestTranscriptionCLI(t *testing.T) {
	t.Setenv("MINIMAX_API_KEY", "test-key")
	dir := t.TempDir()
	audio := filepath.Join(dir, "audio.wav")
	if err := os.WriteFile(audio, []byte("audio"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"json", "verbose_json", "srt", "vtt", "stream"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseMultipartForm(1024); err != nil {
					t.Error(err)
					return
				}
				defer r.MultipartForm.RemoveAll()
				if r.Header.Get("Language") != "zh" || r.Header.Get("Authorization") != "Bearer test-key" {
					t.Error("wrong headers")
				}
				switch mode {
				case "srt":
					w.Header().Set("Content-Type", "text/plain")
					fmt.Fprint(w, "1\n00:00:00,000 --> 00:00:01,000\nhello\n")
				case "vtt":
					w.Header().Set("Content-Type", "text/vtt")
					fmt.Fprint(w, "WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nhello\n")
				case "stream":
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "data: {\"index\":0,\"delta\":\"hello\",\"finish\":true,\"duration\":1}\n\n")
				default:
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"text":"hello","duration":1,"n_speakers":1,"segments":[]}`)
				}
			}))
			defer server.Close()
			var stdout, stderr bytes.Buffer
			args := []string{"-file", audio, "-language", "zh", "-base-url", server.URL}
			if mode == "stream" {
				args = append(args, "-stream")
			} else {
				args = append(args, "-format", mode)
			}
			output := filepath.Join(dir, mode+".txt")
			if mode == "vtt" {
				args = append(args, "-output", output)
			}
			if err := run(args, &stdout, &stderr); err != nil {
				t.Fatal(err)
			}
			body := stdout.Bytes()
			if mode == "vtt" {
				var err error
				body, err = os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
				if stdout.Len() != 0 {
					t.Fatal("unexpected stdout")
				}
			}
			if mode == "json" || mode == "verbose_json" {
				var result minimax.SpeechToTextResponse
				if err := json.Unmarshal(body, &result); err != nil || result.Text != "hello" || result.Duration == nil || *result.Duration != 1 {
					t.Fatalf("result: %s %v", body, err)
				}
			} else if mode == "stream" {
				var chunk minimax.SpeechToTextChunk
				if err := json.Unmarshal(body, &chunk); err != nil || !chunk.Finish || chunk.Delta != "hello" {
					t.Fatalf("chunk: %s %v", body, err)
				}
			} else if !strings.Contains(string(body), "hello\n") {
				t.Fatalf("subtitles: %q", body)
			}
		})
	}
}

func TestTranscriptionCLIValidation(t *testing.T) {
	var stderr bytes.Buffer
	if err := run([]string{"-h"}, io.Discard, &stderr); !errors.Is(err, flag.ErrHelp) || !strings.Contains(stderr.String(), "verbose_json") {
		t.Fatalf("help: %s %v", stderr.String(), err)
	}
	for _, args := range [][]string{nil, {"-file", "x.wav", "-timeout", "0"}, {"extra"}} {
		if err := run(args, io.Discard, io.Discard); err == nil {
			t.Fatal("accepted", args)
		}
	}
	t.Setenv("MINIMAX_API_KEY", "")
	if err := run([]string{"-file", "x.wav"}, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "MINIMAX_API_KEY") {
		t.Fatal(err)
	}
}
