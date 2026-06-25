package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseOptions(t *testing.T) {
	t.Run("top level help", func(t *testing.T) {
		var stderr bytes.Buffer
		_, err := parseOptions([]string{"-h"}, &stderr)
		if !errors.Is(err, flag.ErrHelp) || !strings.Contains(stderr.String(), "Modes:") {
			t.Fatalf("parseOptions(-h) err=%v output=%q, want help", err, stderr.String())
		}
	})

	t.Run("generate mode parses flags", func(t *testing.T) {
		opts, err := parseOptions([]string{
			"generate",
			"-api-key", "test-key",
			"-base-url", "http://example.test",
			"-model", " music-2.6-free ",
			"-prompt", " bright ",
			"-lyrics", " [Verse]\nhi ",
			"-output-format", " hex ",
			"-sample-rate", "32000",
			"-bitrate", "128000",
			"-audio-format", " wav ",
			"-aigc-watermark",
			"-lyrics-optimizer",
			"-timeout", "3s",
		}, ioDiscard{})
		if err != nil {
			t.Fatalf("parseOptions() error = %v, want nil", err)
		}
		if opts.mode != "generate" || opts.model != "music-2.6-free" || opts.prompt != "bright" || opts.lyrics != "[Verse]\nhi" ||
			opts.outputFormat != "hex" || opts.sampleRate != 32000 || opts.bitrate != 128000 || opts.audioFormat != "wav" ||
			!opts.aigcWatermark || !opts.lyricsOptimizer || opts.timeout != 3*time.Second {
			t.Fatalf("opts = %+v, want parsed generate flags", opts)
		}
	})

	t.Run("generate mode defaults to official music base URL", func(t *testing.T) {
		opts, err := parseOptions([]string{"generate"}, ioDiscard{})
		if err != nil {
			t.Fatalf("parseOptions() error = %v, want nil", err)
		}
		if defaultBaseURL != "https://api.minimaxi.com" {
			t.Fatalf("defaultBaseURL = %q, want official music base URL", defaultBaseURL)
		}
		if opts.baseURL != defaultBaseURL {
			t.Fatalf("opts.baseURL = %q, want %q", opts.baseURL, defaultBaseURL)
		}
	})

	t.Run("cover defaults cover model", func(t *testing.T) {
		opts, err := parseOptions([]string{"cover", "-api-key", "test-key", "-audio-url", "https://example.com/a.mp3"}, ioDiscard{})
		if err != nil {
			t.Fatalf("parseOptions() error = %v, want nil", err)
		}
		if opts.model != defaultCoverModel {
			t.Fatalf("opts.model = %q, want default cover model", opts.model)
		}
	})

	t.Run("lyrics mode parses lyrics mode", func(t *testing.T) {
		opts, err := parseOptions([]string{"lyrics", "-api-key", "test-key", "-lyrics-mode", " edit ", "-lyrics", "old"}, ioDiscard{})
		if err != nil {
			t.Fatalf("parseOptions() error = %v, want nil", err)
		}
		if opts.lyricsMode != "edit" {
			t.Fatalf("opts.lyricsMode = %q, want edit", opts.lyricsMode)
		}
	})

	t.Run("unknown mode fails", func(t *testing.T) {
		_, err := parseOptions([]string{"unknown"}, ioDiscard{})
		if err == nil || !strings.Contains(err.Error(), "unknown mode") {
			t.Fatalf("parseOptions() error = %v, want unknown mode", err)
		}
	})

	t.Run("invalid timeout fails", func(t *testing.T) {
		_, err := parseOptions([]string{"generate", "-timeout", "0s"}, ioDiscard{})
		if err == nil || !strings.Contains(err.Error(), "timeout") {
			t.Fatalf("parseOptions() error = %v, want timeout validation", err)
		}
	})

	t.Run("invalid numeric values fail", func(t *testing.T) {
		_, err := parseOptions([]string{"generate", "-sample-rate", "-1"}, ioDiscard{})
		if err == nil || !strings.Contains(err.Error(), "sample-rate") {
			t.Fatalf("parseOptions() sample-rate error = %v, want validation", err)
		}
		_, err = parseOptions([]string{"generate", "-bitrate", "-1"}, ioDiscard{})
		if err == nil || !strings.Contains(err.Error(), "bitrate") {
			t.Fatalf("parseOptions() bitrate error = %v, want validation", err)
		}
	})
}

func TestRunModes(t *testing.T) {
	t.Run("lyrics mode prints generated lyrics", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/lyrics_generation" {
				t.Fatalf("path = %s, want /v1/lyrics_generation", r.URL.Path)
			}
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload["mode"] != "write_full_song" || payload["prompt"] != "summer song" {
				t.Fatalf("payload = %+v, want lyrics request", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"song_title":"Summer","style_tags":"Pop","lyrics":"[Verse]\nhello","base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		var stdout bytes.Buffer
		err := run(options{
			mode:       "lyrics",
			apiKey:     "test-key",
			baseURL:    srv.URL,
			lyricsMode: "write_full_song",
			prompt:     "summer song",
			timeout:    time.Second,
			asJSON:     true,
		}, &stdout)
		if err != nil {
			t.Fatalf("run() error = %v, want nil", err)
		}
		if !strings.Contains(stdout.String(), `"song_title": "Summer"`) ||
			!strings.Contains(stdout.String(), "song_title=Summer") ||
			!strings.Contains(stdout.String(), "style_tags=Pop") {
			t.Fatalf("stdout = %q, want lyrics summary", stdout.String())
		}
	})

	t.Run("generate mode saves hex audio", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/music_generation" {
				t.Fatalf("path = %s, want /v1/music_generation", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"audio":"68656c6c6f","status":2},"extra_info":{"music_duration":10,"music_size":5},"trace_id":"trace-1","base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		output := filepath.Join(t.TempDir(), "song.mp3")
		var stdout bytes.Buffer
		err := run(options{
			mode:         "generate",
			apiKey:       "test-key",
			baseURL:      srv.URL,
			model:        "music-2.6-free",
			prompt:       "bright",
			lyrics:       "[Verse]\nhello",
			outputFormat: "hex",
			audioFormat:  "mp3",
			output:       output,
			timeout:      time.Second,
			asJSON:       true,
		}, &stdout)
		if err != nil {
			t.Fatalf("run() error = %v, want nil", err)
		}
		data, err := os.ReadFile(output)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(data) != "hello" || !strings.Contains(stdout.String(), `"audio": "68656c6c6f"`) || !strings.Contains(stdout.String(), "saved="+output) {
			t.Fatalf("data=%q stdout=%q, want saved decoded audio", string(data), stdout.String())
		}
	})

	t.Run("preprocess mode prints feature id", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/music_cover_preprocess" {
				t.Fatalf("path = %s, want /v1/music_cover_preprocess", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"cover_feature_id":"feature-1","formatted_lyrics":"[Verse]\nline","audio_duration":8.5,"base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		var stdout bytes.Buffer
		err := run(options{
			mode:     "preprocess",
			apiKey:   "test-key",
			baseURL:  srv.URL,
			model:    "music-cover",
			audioURL: "https://example.com/a.mp3",
			timeout:  time.Second,
			asJSON:   true,
		}, &stdout)
		if err != nil {
			t.Fatalf("run() error = %v, want nil", err)
		}
		if !strings.Contains(stdout.String(), `"cover_feature_id": "feature-1"`) || !strings.Contains(stdout.String(), "cover_feature_id=feature-1") {
			t.Fatalf("stdout = %q, want feature id", stdout.String())
		}
	})

	t.Run("cover mode sends cover feature id", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload["cover_feature_id"] != "feature-1" || payload["lyrics"] != "[Verse]\nedited" {
				t.Fatalf("payload = %+v, want two-step cover request", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"audio":"https://cdn.example.com/cover.mp3","status":2},"base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		var stdout bytes.Buffer
		err := run(options{
			mode:           "cover",
			apiKey:         "test-key",
			baseURL:        srv.URL,
			model:          "music-cover-free",
			prompt:         "jazz lounge",
			lyrics:         "[Verse]\nedited",
			coverFeatureID: "feature-1",
			outputFormat:   "url",
			audioFormat:    "mp3",
			timeout:        time.Second,
		}, &stdout)
		if err != nil {
			t.Fatalf("run() error = %v, want nil", err)
		}
		if !strings.Contains(stdout.String(), "audio=https://cdn.example.com/cover.mp3") {
			t.Fatalf("stdout = %q, want cover audio URL", stdout.String())
		}
	})

	t.Run("missing api key fails", func(t *testing.T) {
		var stdout bytes.Buffer
		err := run(options{mode: "generate", baseURL: "http://example.test", timeout: time.Second}, &stdout)
		if err == nil || !strings.Contains(err.Error(), "missing API key") {
			t.Fatalf("run() error = %v, want missing API key", err)
		}
	})

	t.Run("empty base url fails", func(t *testing.T) {
		var stdout bytes.Buffer
		err := run(options{mode: "generate", apiKey: "test-key", timeout: time.Second}, &stdout)
		if err == nil || !strings.Contains(err.Error(), "base-url") {
			t.Fatalf("run() error = %v, want base-url validation", err)
		}
	})

	t.Run("unknown mode fails", func(t *testing.T) {
		var stdout bytes.Buffer
		err := run(options{mode: "unknown", apiKey: "test-key", baseURL: "http://example.test", timeout: time.Second}, &stdout)
		if err == nil || !strings.Contains(err.Error(), "unknown mode") {
			t.Fatalf("run() error = %v, want unknown mode", err)
		}
	})

	t.Run("reads lyrics file", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload["lyrics"] != "[Verse]\nfrom file" {
				t.Fatalf("payload lyrics = %q, want file lyrics", payload["lyrics"])
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"audio":"6869","status":2},"base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		lyricsFile := filepath.Join(t.TempDir(), "lyrics.txt")
		if err := os.WriteFile(lyricsFile, []byte(" [Verse]\nfrom file "), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		var stdout bytes.Buffer
		err := run(options{
			mode:         "generate",
			apiKey:       "test-key",
			baseURL:      srv.URL,
			model:        "music-2.6-free",
			prompt:       "bright",
			lyricsFile:   lyricsFile,
			outputFormat: "hex",
			timeout:      time.Second,
		}, &stdout)
		if err != nil {
			t.Fatalf("run() error = %v, want nil", err)
		}
	})
}

func TestSaveAudio(t *testing.T) {
	t.Run("hex", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "audio.mp3")
		if err := saveAudio(t.Context(), hex.EncodeToString([]byte("hello")), path); err != nil {
			t.Fatalf("saveAudio() error = %v, want nil", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(data) != "hello" {
			t.Fatalf("data = %q, want hello", string(data))
		}
	})

	t.Run("url", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("audio"))
		}))
		defer srv.Close()

		path := filepath.Join(t.TempDir(), "audio.mp3")
		if err := saveAudio(t.Context(), srv.URL+"/audio.mp3", path); err != nil {
			t.Fatalf("saveAudio() error = %v, want nil", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(data) != "audio" {
			t.Fatalf("data = %q, want audio", string(data))
		}
	})

	t.Run("empty audio fails", func(t *testing.T) {
		err := saveAudio(t.Context(), "", filepath.Join(t.TempDir(), "audio.mp3"))
		if err == nil || !strings.Contains(err.Error(), "empty") {
			t.Fatalf("saveAudio() error = %v, want empty audio", err)
		}
	})

	t.Run("bad hex fails", func(t *testing.T) {
		err := saveAudio(t.Context(), "not-hex", filepath.Join(t.TempDir(), "audio.mp3"))
		if err == nil || !strings.Contains(err.Error(), "decode hex") {
			t.Fatalf("saveAudio() error = %v, want decode error", err)
		}
	})

	t.Run("url status fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "not found", http.StatusNotFound)
		}))
		defer srv.Close()

		err := saveAudio(t.Context(), srv.URL+"/missing.mp3", filepath.Join(t.TempDir(), "audio.mp3"))
		if err == nil || !strings.Contains(err.Error(), "status 404") {
			t.Fatalf("saveAudio() error = %v, want status error", err)
		}
	})
}

func TestHelpers(t *testing.T) {
	t.Run("build audio setting nil when empty", func(t *testing.T) {
		if got := buildAudioSetting(options{}); got != nil {
			t.Fatalf("buildAudioSetting() = %+v, want nil", got)
		}
	})

	t.Run("format optional values", func(t *testing.T) {
		if got := formatOptionalInt(nil); got != "-" {
			t.Fatalf("formatOptionalInt(nil) = %q, want -", got)
		}
		value := 42
		if got := formatOptionalInt(&value); got != "42" {
			t.Fatalf("formatOptionalInt() = %q, want 42", got)
		}
		if got := formatOptionalFloat(nil); got != "-" {
			t.Fatalf("formatOptionalFloat(nil) = %q, want -", got)
		}
		duration := 1.25
		if got := formatOptionalFloat(&duration); got != "1.250" {
			t.Fatalf("formatOptionalFloat() = %q, want 1.250", got)
		}
	})

	t.Run("env helpers parse valid values and fall back", func(t *testing.T) {
		t.Setenv("MINIMAX_TEST_STRING", "value")
		if got := envOrDefault("MINIMAX_TEST_STRING", "fallback"); got != "value" {
			t.Fatalf("envOrDefault() = %q, want value", got)
		}
		if got := envOrDefault("MINIMAX_TEST_STRING_MISSING", "fallback"); got != "fallback" {
			t.Fatalf("envOrDefault() = %q, want fallback", got)
		}

		t.Setenv("MINIMAX_TEST_BOOL", "true")
		if got := envBoolOrDefault("MINIMAX_TEST_BOOL", false); !got {
			t.Fatal("envBoolOrDefault() = false, want true")
		}
		t.Setenv("MINIMAX_TEST_BOOL", "invalid")
		if got := envBoolOrDefault("MINIMAX_TEST_BOOL", true); !got {
			t.Fatal("envBoolOrDefault() = false, want fallback true")
		}

		t.Setenv("MINIMAX_TEST_DURATION", "250ms")
		if got := envDurationOrDefault("MINIMAX_TEST_DURATION", time.Second); got != 250*time.Millisecond {
			t.Fatalf("envDurationOrDefault() = %s, want 250ms", got)
		}
		t.Setenv("MINIMAX_TEST_DURATION", "invalid")
		if got := envDurationOrDefault("MINIMAX_TEST_DURATION", time.Second); got != time.Second {
			t.Fatalf("envDurationOrDefault() = %s, want fallback", got)
		}

		t.Setenv("MINIMAX_TEST_INT", "12")
		if got := envIntOrDefault("MINIMAX_TEST_INT", 7); got != 12 {
			t.Fatalf("envIntOrDefault() = %d, want 12", got)
		}
		t.Setenv("MINIMAX_TEST_INT", "invalid")
		if got := envIntOrDefault("MINIMAX_TEST_INT", 7); got != 7 {
			t.Fatalf("envIntOrDefault() = %d, want fallback", got)
		}
	})
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
