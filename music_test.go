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

func TestMusicGenerate(t *testing.T) {
	t.Parallel()

	t.Run("success maps song response and request JSON", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != defaultMusicGenerationPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultMusicGenerationPath)
			}
			var payload MusicGenerateRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.Model != "music-2.6" || payload.Prompt != "Mandopop, bright" || payload.Lyrics != "[Verse]\nhello" {
				t.Fatalf("payload model/prompt/lyrics = %q/%q/%q, want trimmed song request", payload.Model, payload.Prompt, payload.Lyrics)
			}
			if payload.OutputFormat != "url" {
				t.Fatalf("payload.OutputFormat = %q, want url", payload.OutputFormat)
			}
			if payload.AudioSetting == nil || payload.AudioSetting.SampleRate == nil || *payload.AudioSetting.SampleRate != 44100 ||
				payload.AudioSetting.Bitrate == nil || *payload.AudioSetting.Bitrate != 256000 || payload.AudioSetting.Format != "mp3" {
				t.Fatalf("payload.AudioSetting = %+v, want full audio setting", payload.AudioSetting)
			}
			if payload.AIGCWatermark == nil || *payload.AIGCWatermark {
				t.Fatalf("payload.AIGCWatermark = %v, want explicit false", payload.AIGCWatermark)
			}

			w.Header().Set("X-Trace-ID", "trace-music-header")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"audio":"https://cdn.example.com/song.mp3","status":2},"trace_id":"trace-body","extra_info":{"music_duration":25364,"music_sample_rate":44100,"music_channel":2,"bitrate":256000,"music_size":813651},"analysis_info":null,"extra":"kept","base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newMusicTestClient(t, srv)
		response, err := client.Music.Generate(context.Background(), MusicGenerateRequest{
			Model:         " music-2.6 ",
			Prompt:        " Mandopop, bright ",
			Lyrics:        "[Verse]\nhello",
			OutputFormat:  " url ",
			AudioSetting:  &MusicAudioSetting{SampleRate: new(44100), Bitrate: new(256000), Format: " mp3 "},
			AIGCWatermark: new(false),
		})
		if err != nil {
			t.Fatalf("Generate() error = %v, want nil", err)
		}
		if response.Audio != "https://cdn.example.com/song.mp3" || response.Status == nil || *response.Status != 2 {
			t.Fatalf("response audio/status = %q/%v, want URL/status 2", response.Audio, response.Status)
		}
		if response.TraceID != "trace-body" || response.ResponseMeta.TraceID != "trace-music-header" {
			t.Fatalf("trace fields = response %q meta %q, want body/header traces", response.TraceID, response.ResponseMeta.TraceID)
		}
		if response.ExtraInfo.MusicDuration == nil || *response.ExtraInfo.MusicDuration != 25364 ||
			response.ExtraInfo.MusicSampleRate == nil || *response.ExtraInfo.MusicSampleRate != 44100 ||
			response.ExtraInfo.MusicChannel == nil || *response.ExtraInfo.MusicChannel != 2 ||
			response.ExtraInfo.Bitrate == nil || *response.ExtraInfo.Bitrate != 256000 ||
			response.ExtraInfo.MusicSize == nil || *response.ExtraInfo.MusicSize != 813651 {
			t.Fatalf("response.ExtraInfo = %+v, want mapped metadata", response.ExtraInfo)
		}
		if _, ok := response.Raw["extra"]; !ok {
			t.Fatalf("response.Raw missing extra field: %+v", response.Raw)
		}
	})

	t.Run("instrumental generation allows empty lyrics with prompt", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload MusicGenerateRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.IsInstrumental == nil || !*payload.IsInstrumental || payload.Lyrics != "" {
				t.Fatalf("payload instrumental/lyrics = %v/%q, want instrumental without lyrics", payload.IsInstrumental, payload.Lyrics)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"audio":"68656c6c6f","status":2},"base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newMusicTestClient(t, srv)
		response, err := client.Music.Generate(context.Background(), MusicGenerateRequest{
			Model:          string(MusicModelV26Free),
			Prompt:         "cinematic ambient synthwave",
			IsInstrumental: new(true),
		})
		if err != nil {
			t.Fatalf("Generate() error = %v, want nil", err)
		}
		if response.Audio != "68656c6c6f" {
			t.Fatalf("response.Audio = %q, want hex audio", response.Audio)
		}
	})

	t.Run("lyrics optimizer allows empty lyrics with prompt", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload MusicGenerateRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.LyricsOptimizer == nil || !*payload.LyricsOptimizer || payload.Lyrics != "" {
				t.Fatalf("payload lyrics_optimizer/lyrics = %v/%q, want optimizer without lyrics", payload.LyricsOptimizer, payload.Lyrics)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"audio":"https://cdn.example.com/optimized.mp3","status":2},"base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newMusicTestClient(t, srv)
		_, err := client.Music.Generate(context.Background(), MusicGenerateRequest{
			Model:           string(MusicModelV26Free),
			Prompt:          "upbeat chiptune theme",
			LyricsOptimizer: new(true),
		})
		if err != nil {
			t.Fatalf("Generate() error = %v, want nil", err)
		}
	})

	t.Run("one-step cover requires one audio source", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload MusicGenerateRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.Model != "music-cover-free" || payload.AudioURL != "https://example.com/original.mp3" || payload.AudioBase64 != "" || payload.CoverFeatureID != "" {
				t.Fatalf("payload cover source fields = model %q url %q b64 %q feature %q, want one-step cover", payload.Model, payload.AudioURL, payload.AudioBase64, payload.CoverFeatureID)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"audio":"https://cdn.example.com/cover.mp3","status":2},"base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newMusicTestClient(t, srv)
		response, err := client.Music.Generate(context.Background(), MusicGenerateRequest{
			Model:        "music-cover-free",
			AudioURL:     " https://example.com/original.mp3 ",
			Prompt:       "jazz lounge late night",
			OutputFormat: string(MusicOutputFormatURL),
		})
		if err != nil {
			t.Fatalf("Generate() error = %v, want nil", err)
		}
		if response.Audio != "https://cdn.example.com/cover.mp3" {
			t.Fatalf("response.Audio = %q, want cover URL", response.Audio)
		}
	})

	t.Run("two-step cover requires lyrics with feature id", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload MusicGenerateRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.CoverFeatureID != "feature-123" || payload.Lyrics == "" {
				t.Fatalf("payload feature/lyrics = %q/%q, want two-step cover", payload.CoverFeatureID, payload.Lyrics)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"audio":"https://cdn.example.com/two-step-cover.mp3","status":2},"base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newMusicTestClient(t, srv)
		_, err := client.Music.Generate(context.Background(), MusicGenerateRequest{
			Model:          string(MusicModelCover),
			CoverFeatureID: " feature-123 ",
			Lyrics:         "[Verse]\nedited lyric",
			Prompt:         "jazz lounge late night",
			OutputFormat:   string(MusicOutputFormatURL),
		})
		if err != nil {
			t.Fatalf("Generate() error = %v, want nil", err)
		}
	})

	validationCases := []struct {
		name    string
		request MusicGenerateRequest
		want    string
	}{
		{name: "empty model", request: MusicGenerateRequest{Lyrics: "hello"}, want: "model is empty"},
		{name: "unsupported model", request: MusicGenerateRequest{Model: "music-1", Lyrics: "hello"}, want: "model is not supported"},
		{name: "stream unsupported", request: MusicGenerateRequest{Model: string(MusicModelV26), Lyrics: "hello", Stream: new(true)}, want: "stream=true is not supported"},
		{name: "invalid output format", request: MusicGenerateRequest{Model: string(MusicModelV26), Lyrics: "hello", OutputFormat: "binary"}, want: "output_format"},
		{name: "invalid audio format", request: MusicGenerateRequest{Model: string(MusicModelV26), Lyrics: "hello", AudioSetting: &MusicAudioSetting{Format: "flac"}}, want: "audio_setting.format"},
		{name: "song lyrics required", request: MusicGenerateRequest{Model: string(MusicModelV26)}, want: "lyrics is empty"},
		{name: "instrumental prompt required", request: MusicGenerateRequest{Model: string(MusicModelV26), IsInstrumental: new(true)}, want: "prompt is empty"},
		{name: "optimizer prompt required", request: MusicGenerateRequest{Model: string(MusicModelV26), LyricsOptimizer: new(true)}, want: "prompt is empty"},
		{name: "song rejects cover source", request: MusicGenerateRequest{Model: string(MusicModelV26), Lyrics: "hello", AudioURL: "https://example.com/a.mp3"}, want: "require a music-cover model"},
		{name: "cover source required", request: MusicGenerateRequest{Model: string(MusicModelCover), Prompt: "jazz lounge"}, want: "exactly one"},
		{name: "cover sources mutually exclusive", request: MusicGenerateRequest{Model: string(MusicModelCover), Prompt: "jazz lounge", AudioURL: "https://example.com/a.mp3", AudioBase64: "AAAA"}, want: "exactly one"},
		{name: "cover prompt required", request: MusicGenerateRequest{Model: string(MusicModelCover), AudioURL: "https://example.com/a.mp3"}, want: "prompt is empty"},
		{name: "cover feature requires lyrics", request: MusicGenerateRequest{Model: string(MusicModelCover), Prompt: "jazz lounge", CoverFeatureID: "feature-1"}, want: "lyrics is empty"},
		{name: "cover rejects lyrics optimizer", request: MusicGenerateRequest{Model: string(MusicModelCover), Prompt: "jazz lounge", AudioURL: "https://example.com/a.mp3", LyricsOptimizer: new(true)}, want: "lyrics_optimizer"},
		{name: "cover rejects instrumental", request: MusicGenerateRequest{Model: string(MusicModelCover), Prompt: "jazz lounge", AudioURL: "https://example.com/a.mp3", IsInstrumental: new(true)}, want: "is_instrumental"},
	}
	for _, tc := range validationCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, err := NewClient(Config{})
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			_, err = client.Music.Generate(context.Background(), tc.request)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Generate() error = %v, want containing %q", err, tc.want)
			}
		})
	}

	t.Run("base_resp error returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":2013,"status_msg":"invalid lyrics"}}`))
		}))
		defer srv.Close()

		client := newMusicTestClient(t, srv)
		_, err := client.Music.Generate(context.Background(), MusicGenerateRequest{Model: string(MusicModelV26), Lyrics: "hello"})
		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 2013 || apiErr.StatusMsg != "invalid lyrics" {
			t.Fatalf("Generate() error = %v, want APIError 2013 invalid lyrics", err)
		}
	})

	t.Run("http error returns unified api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		client := newMusicTestClient(t, srv)
		_, err := client.Music.Generate(context.Background(), MusicGenerateRequest{Model: string(MusicModelV26), Lyrics: "hello"})
		var apiErr *protocol.APIError
		if !errors.As(err, &apiErr) || apiErr.HTTPStatus != http.StatusServiceUnavailable {
			t.Fatalf("Generate() error = %v, want HTTP APIError 503", err)
		}
	})

	t.Run("malformed json returns decode error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{`))
		}))
		defer srv.Close()

		client := newMusicTestClient(t, srv)
		_, err := client.Music.Generate(context.Background(), MusicGenerateRequest{Model: string(MusicModelV26), Lyrics: "hello"})
		if err == nil || !strings.Contains(err.Error(), "decode response body") {
			t.Fatalf("Generate() error = %v, want decode error", err)
		}
	})
}

func TestMusicPreprocessCover(t *testing.T) {
	t.Parallel()

	t.Run("success maps response", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != defaultMusicCoverPreprocessPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultMusicCoverPreprocessPath)
			}
			var payload MusicCoverPreprocessRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.Model != "music-cover" || payload.AudioURL != "https://example.com/song.mp3" || payload.AudioBase64 != "" {
				t.Fatalf("payload = %+v, want trimmed URL preprocess request", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"cover_feature_id":"feature-123","formatted_lyrics":"[Verse]\nline","structure_result":"{\"num_segments\":1}","audio_duration":90.5,"trace_id":"trace-cover","extra":"kept","base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newMusicTestClient(t, srv)
		response, err := client.Music.PreprocessCover(context.Background(), MusicCoverPreprocessRequest{
			Model:    " music-cover ",
			AudioURL: " https://example.com/song.mp3 ",
		})
		if err != nil {
			t.Fatalf("PreprocessCover() error = %v, want nil", err)
		}
		if response.CoverFeatureID != "feature-123" || response.FormattedLyrics != "[Verse]\nline" ||
			response.StructureResult != "{\"num_segments\":1}" || response.AudioDuration == nil || *response.AudioDuration != 90.5 ||
			response.TraceID != "trace-cover" {
			t.Fatalf("response = %+v, want mapped preprocess response", response)
		}
		if _, ok := response.Raw["extra"]; !ok {
			t.Fatalf("response.Raw missing extra field: %+v", response.Raw)
		}
	})

	validationCases := []struct {
		name    string
		request MusicCoverPreprocessRequest
		want    string
	}{
		{name: "empty model", request: MusicCoverPreprocessRequest{AudioURL: "https://example.com/a.mp3"}, want: "model is empty"},
		{name: "unsupported model", request: MusicCoverPreprocessRequest{Model: "music-cover-free", AudioURL: "https://example.com/a.mp3"}, want: "model must be"},
		{name: "missing source", request: MusicCoverPreprocessRequest{Model: string(MusicModelCover)}, want: "exactly one"},
		{name: "two sources", request: MusicCoverPreprocessRequest{Model: string(MusicModelCover), AudioURL: "https://example.com/a.mp3", AudioBase64: "AAAA"}, want: "exactly one"},
	}
	for _, tc := range validationCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, err := NewClient(Config{})
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			_, err = client.Music.PreprocessCover(context.Background(), tc.request)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("PreprocessCover() error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestMusicGenerateLyrics(t *testing.T) {
	t.Parallel()

	t.Run("success maps lyrics", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != defaultLyricsGenerationPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, defaultLyricsGenerationPath)
			}
			var payload LyricsGenerateRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.Mode != "write_full_song" || payload.Prompt != "summer beach pop" || payload.Title != "Summer Promise" {
				t.Fatalf("payload = %+v, want trimmed lyrics request", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"song_title":"Summer Promise","style_tags":"Pop, Beach","lyrics":"[Intro]\nhello","extra":"kept","base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newMusicTestClient(t, srv)
		response, err := client.Music.GenerateLyrics(context.Background(), LyricsGenerateRequest{
			Mode:   " write_full_song ",
			Prompt: " summer beach pop ",
			Title:  " Summer Promise ",
		})
		if err != nil {
			t.Fatalf("GenerateLyrics() error = %v, want nil", err)
		}
		if response.SongTitle != "Summer Promise" || response.StyleTags != "Pop, Beach" || response.Lyrics != "[Intro]\nhello" {
			t.Fatalf("response = %+v, want mapped lyrics response", response)
		}
		if _, ok := response.Raw["extra"]; !ok {
			t.Fatalf("response.Raw missing extra field: %+v", response.Raw)
		}
	})

	t.Run("edit mode sends existing lyrics", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload LyricsGenerateRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.Mode != "edit" || payload.Lyrics != "[Verse]\nold lyric" {
				t.Fatalf("payload = %+v, want edit lyrics", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"song_title":"Edited","style_tags":"Pop","lyrics":"[Verse]\nnew lyric","base_resp":{"status_code":0,"status_msg":"success"}}`))
		}))
		defer srv.Close()

		client := newMusicTestClient(t, srv)
		_, err := client.Music.GenerateLyrics(context.Background(), LyricsGenerateRequest{
			Mode:   string(LyricsModeEdit),
			Prompt: "make it brighter",
			Lyrics: "[Verse]\nold lyric",
		})
		if err != nil {
			t.Fatalf("GenerateLyrics() error = %v, want nil", err)
		}
	})

	validationCases := []struct {
		name    string
		request LyricsGenerateRequest
		want    string
	}{
		{name: "empty mode", request: LyricsGenerateRequest{Prompt: "hello"}, want: "mode is empty"},
		{name: "invalid mode", request: LyricsGenerateRequest{Mode: "rewrite"}, want: "mode must be"},
		{name: "edit lyrics required", request: LyricsGenerateRequest{Mode: string(LyricsModeEdit), Prompt: "edit"}, want: "lyrics is empty"},
	}
	for _, tc := range validationCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, err := NewClient(Config{})
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			_, err = client.Music.GenerateLyrics(context.Background(), tc.request)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("GenerateLyrics() error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestMusicServiceUninitialized(t *testing.T) {
	t.Parallel()

	var service *MusicService
	if _, err := service.Generate(context.Background(), MusicGenerateRequest{}); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("Generate() error = %v, want uninitialized error", err)
	}
	if _, err := service.PreprocessCover(context.Background(), MusicCoverPreprocessRequest{}); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("PreprocessCover() error = %v, want uninitialized error", err)
	}
	if _, err := service.GenerateLyrics(context.Background(), LyricsGenerateRequest{}); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("GenerateLyrics() error = %v, want uninitialized error", err)
	}
}

func TestMusicContextCancellation(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	client := newMusicTestClient(t, srv)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Music.Generate(ctx, MusicGenerateRequest{Model: string(MusicModelV26), Lyrics: "hello"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Generate() error = %v, want context.Canceled", err)
	}
}

func newMusicTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()

	client, err := NewClient(Config{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Retry: transport.RetryConfig{
			MaxAttempts: 1,
		},
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}
