package minimax

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GizClaw/minimax-go/internal/transport"
)

func transcriptionRequest() SpeechToTextRequest {
	return SpeechToTextRequest{File: SpeechToTextFile{Name: "sample.wav", Data: []byte("audio")}}
}

func transcriptionClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := NewClient(Config{BaseURL: server.URL, APIKey: "test-key", Retry: transport.RetryConfig{MaxAttempts: 3, Sleep: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }}})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestSpeechToTextFormats(t *testing.T) {
	for _, tc := range []struct {
		format            SpeechToTextFormat
		contentType, body string
	}{
		{SpeechToTextFormatJSON, "application/json", `{"text":" hello ","duration":0,"trace_id":"body-trace"}`},
		{SpeechToTextFormatVerboseJSON, "application/json", `{"text":"hello","duration":2.3,"n_speakers":1,"segments":[{"id":0,"start":0.1,"end":2.2,"speaker":"S1","text":"hello"}],"trace_id":"body-trace"}`},
		{SpeechToTextFormatSRT, "text/plain", "1\r\n00:00:00,100 --> 00:00:02,200\r\nhello\r\n\r\n"},
		{SpeechToTextFormatVTT, "text/vtt", "WEBVTT\n\n00:00:00.100 --> 00:00:02.200\nhello\n"},
	} {
		t.Run(string(tc.format), func(t *testing.T) {
			client := transcriptionClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/v1/speech_to_text" || r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("language") != "en" || r.Header.Get("Accept") != tc.contentType {
					t.Errorf("wrong request: %s %s %v", r.Method, r.URL, r.Header)
				}
				if err := r.ParseMultipartForm(1024); err != nil {
					t.Error(err)
					return
				}
				defer r.MultipartForm.RemoveAll()
				if len(r.MultipartForm.Value) != 3 || r.FormValue("model") != "asr-1.0" || r.FormValue("stream") != "false" || r.FormValue("response_format") != string(tc.format) {
					t.Errorf("wrong fields: %v", r.MultipartForm.Value)
				}
				file, header, err := r.FormFile("file")
				if err != nil {
					t.Error(err)
					return
				}
				defer file.Close()
				data, err := io.ReadAll(file)
				if err != nil || string(data) != "audio" || header.Filename != "sample.wav" {
					t.Errorf("wrong file: %q %v", data, err)
				}
				w.Header().Set("Content-Type", tc.contentType+"; charset=utf-8")
				w.Header().Set("X-Request-ID", "header-request")
				fmt.Fprint(w, tc.body)
			})
			request := transcriptionRequest()
			request.ResponseFormat = tc.format
			request.Language = "en"
			result, err := client.SpeechToText.Transcribe(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if result.ResponseMeta.HTTPStatus != 200 || result.ResponseMeta.RequestID != "header-request" {
				t.Fatalf("metadata: %+v", result.ResponseMeta)
			}
			if tc.format == SpeechToTextFormatSRT || tc.format == SpeechToTextFormatVTT {
				if result.Subtitles != tc.body || result.Duration != nil {
					t.Fatalf("subtitles: %+v", result)
				}
			} else {
				if result.Duration == nil || result.TraceID != "body-trace" || result.ResponseMeta.TraceID != "body-trace" {
					t.Fatalf("JSON: %+v", result)
				}
				if tc.format == SpeechToTextFormatJSON && (result.Text != " hello " || *result.Duration != 0) {
					t.Fatalf("text/duration: %+v", result)
				}
				if tc.format == SpeechToTextFormatVerboseJSON && (result.NSpeakers == nil || *result.NSpeakers != 1 || len(result.Segments) != 1 || result.Segments[0] != (SpeechToTextSegment{ID: 0, Start: 0.1, End: 2.2, Speaker: "S1", Text: "hello"})) {
					t.Fatalf("verbose: %+v", result)
				}
			}
		})
	}
}

func TestSpeechToTextValidation(t *testing.T) {
	client := transcriptionClient(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid request reached network") })
	for _, tc := range []struct {
		name   string
		change func(*SpeechToTextRequest)
	}{
		{"model", func(r *SpeechToTextRequest) { r.Model = "invalid" }},
		{"format", func(r *SpeechToTextRequest) { r.ResponseFormat = "invalid" }},
		{"empty file", func(r *SpeechToTextRequest) { r.File.Data = nil }},
		{"empty name", func(r *SpeechToTextRequest) { r.File.Name = "" }},
		{"raw pcm", func(r *SpeechToTextRequest) { r.File.Name = "audio.pcm" }},
		{"filename injection", func(r *SpeechToTextRequest) { r.File.Name = "a\r\n.wav" }},
		{"language injection", func(r *SpeechToTextRequest) { r.Language = "en\r\nX: bad" }},
		{"oversize", func(r *SpeechToTextRequest) { r.File.Data = make([]byte, SpeechToTextMaxFileBytes+1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := transcriptionRequest()
			tc.change(&r)
			if _, err := client.SpeechToText.Transcribe(context.Background(), r); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	for _, format := range []SpeechToTextFormat{SpeechToTextFormatVerboseJSON, SpeechToTextFormatSRT, SpeechToTextFormatVTT} {
		r := transcriptionRequest()
		r.ResponseFormat = format
		if _, err := client.SpeechToText.OpenStream(context.Background(), r); err == nil {
			t.Fatal("accepted incompatible stream format")
		}
	}
	var service *SpeechToTextService
	if _, err := service.Transcribe(context.Background(), transcriptionRequest()); err == nil {
		t.Fatal("nil service")
	}
	r := transcriptionRequest()
	r.File.Data = make([]byte, SpeechToTextMaxFileBytes)
	if _, _, err := client.SpeechToText.prepare(r, false); err != nil {
		t.Fatal("exact size limit:", err)
	}
	for _, ext := range []string{"wav", "aiff", "flac", "m4a", "mp3", "aac", "opus", "ogg", "WAV"} {
		r := transcriptionRequest()
		r.File.Name = "a." + ext
		if _, _, err := client.SpeechToText.prepare(r, false); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSpeechToTextProtocolFailures(t *testing.T) {
	for _, tc := range []struct {
		name, contentType, body string
		verbose                 bool
	}{
		{"wrong content type", "text/html", "failure", false},
		{"malformed", "application/json", "{", false},
		{"null", "application/json", "null", false},
		{"missing text", "application/json", `{"duration":1}`, false},
		{"missing duration", "application/json", `{"text":""}`, false},
		{"negative duration", "application/json", `{"text":"","duration":-1}`, false},
		{"missing verbose fields", "application/json", `{"text":"","duration":1}`, true},
		{"business error", "application/json", `{"base_resp":{"status_code":1008,"status_msg":"balance"}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := transcriptionClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				fmt.Fprint(w, tc.body)
			})
			r := transcriptionRequest()
			if tc.verbose {
				r.ResponseFormat = SpeechToTextFormatVerboseJSON
			}
			if _, err := client.SpeechToText.Transcribe(context.Background(), r); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	client := transcriptionClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"text":"","duration":0,"n_speakers":0,"segments":[]}`)
	})
	r := transcriptionRequest()
	r.ResponseFormat = SpeechToTextFormatVerboseJSON
	result, err := client.SpeechToText.Transcribe(context.Background(), r)
	if err != nil || result.NSpeakers == nil || *result.NSpeakers != 0 {
		t.Fatalf("silent audio: %+v %v", result, err)
	}
}

func TestSpeechToTextErrorsAndRetries(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		for _, status := range []int{400, 401, 402, 413, 422, 429, 500} {
			t.Run(fmt.Sprintf("stream=%t/status=%d", streaming, status), func(t *testing.T) {
				var calls atomic.Int32
				client := transcriptionClient(t, func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					if err := r.ParseMultipartForm(1024); err != nil {
						t.Error(err)
						return
					}
					defer r.MultipartForm.RemoveAll()
					f, _, err := r.FormFile("file")
					if err != nil {
						t.Error(err)
						return
					}
					defer f.Close()
					data, _ := io.ReadAll(f)
					if string(data) != "audio" {
						t.Error("retry did not replay file")
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(status)
					fmt.Fprintf(w, `{"type":"error","error":{"type":"test_error","message":"diagnostic (1004)","http_code":"%d"},"request_id":"error-request"}`, status)
				})
				var err error
				if streaming {
					_, err = client.SpeechToText.OpenStream(context.Background(), transcriptionRequest())
				} else {
					_, err = client.SpeechToText.Transcribe(context.Background(), transcriptionRequest())
				}
				apiErr, ok := errors.AsType[*APIError](err)
				if !ok || apiErr.HTTPStatus != status || apiErr.RequestID != "error-request" || apiErr.Detail == nil || apiErr.Detail.Type != "test_error" || apiErr.Detail.Message != "diagnostic (1004)" || apiErr.Detail.HTTPCode != fmt.Sprint(status) {
					t.Fatalf("error: %#v %v", apiErr, err)
				}
				expected := int32(1)
				if status == 429 || status == 500 {
					expected = 3
				}
				if calls.Load() != expected {
					t.Fatalf("calls=%d", calls.Load())
				}
			})
		}
	}
}

func TestSpeechToTextStream(t *testing.T) {
	var calls atomic.Int32
	client := transcriptionClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.WriteHeader(429)
			return
		}
		if err := r.ParseMultipartForm(1024); err != nil {
			t.Error(err)
			return
		}
		defer r.MultipartForm.RemoveAll()
		if r.FormValue("stream") != "true" || r.FormValue("response_format") != "json" || r.Header.Get("Accept") != "text/event-stream" {
			t.Error("wrong stream request")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ": ping\n\nevent: ping\n\ndata: {\"index\":0,\"delta\":\"你好 \" ,\"finish\":false}\r\n\r\ndata: {\"index\":1,\"delta\":\"world\",\"finish\":false}\n\ndata: {\"index\":2,\"delta\":\"!\",\"finish\":true,\"duration\":0}\n\n")
	})
	stream, err := client.SpeechToText.OpenStream(context.Background(), transcriptionRequest())
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	var text strings.Builder
	for i := range 3 {
		chunk, err := stream.Next()
		if err != nil {
			t.Fatal(err)
		}
		if chunk.Index != i {
			t.Fatal(chunk)
		}
		text.WriteString(chunk.Delta)
		if i == 2 && (!chunk.Finish || chunk.Duration == nil || *chunk.Duration != 0) {
			t.Fatal(chunk)
		}
	}
	if text.String() != "你好 world!" || calls.Load() != 2 {
		t.Fatalf("text=%q calls=%d", text.String(), calls.Load())
	}
	if _, err := stream.Next(); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSpeechToTextStreamFailures(t *testing.T) {
	for _, tc := range []struct {
		name, contentType, body string
		unexpectedEOF, apiError bool
	}{
		{"wrong content type", "application/json", `{"text":"","duration":0}`, false, false},
		{"early EOF", "text/event-stream", "", true, false},
		{"partial then EOF", "text/event-stream", "data: {\"index\":0,\"delta\":\"x\",\"finish\":false}\n\n", true, false},
		{"bad JSON", "text/event-stream", "data: {\n\n", false, false},
		{"missing fields", "text/event-stream", "data: {}\n\n", false, false},
		{"out of order", "text/event-stream", "data: {\"index\":1,\"delta\":\"x\",\"finish\":false}\n\n", false, false},
		{"missing duration", "text/event-stream", "data: {\"index\":0,\"delta\":\"\",\"finish\":true}\n\n", false, false},
		{"unexpected duration", "text/event-stream", "data: {\"index\":0,\"delta\":\"\",\"finish\":false,\"duration\":1}\n\n", false, false},
		{"provider error", "text/event-stream", "data: {\"type\":\"error\",\"error\":{\"type\":\"server_error\",\"message\":\"failed\",\"http_code\":\"500\"}}\n\n", false, true},
		{"business error", "text/event-stream", "data: {\"base_resp\":{\"status_code\":1008,\"status_msg\":\"balance\"}}\n\n", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			client := transcriptionClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", tc.contentType)
				fmt.Fprint(w, tc.body)
			})
			stream, err := client.SpeechToText.OpenStream(context.Background(), transcriptionRequest())
			if err == nil {
				defer stream.Close()
				for range 3 {
					_, err = stream.Next()
					if err != nil {
						break
					}
				}
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if tc.unexpectedEOF && !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatal(err)
			}
			if tc.apiError {
				if _, ok := errors.AsType[*APIError](err); !ok {
					t.Fatal(err)
				}
			}
			if calls.Load() != 1 {
				t.Fatal("retried stream", calls.Load())
			}
		})
	}
}

func TestSpeechToTextCancellation(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		for _, deadline := range []bool{false, true} {
			t.Run(fmt.Sprintf("stream=%t/deadline=%t", streaming, deadline), func(t *testing.T) {
				started := make(chan struct{})
				client := transcriptionClient(t, func(w http.ResponseWriter, r *http.Request) {
					io.Copy(io.Discard, r.Body)
					close(started)
					if streaming {
						w.Header().Set("Content-Type", "text/event-stream")
						w.(http.Flusher).Flush()
					}
					<-r.Context().Done()
				})
				ctx, cancel := context.WithCancel(context.Background())
				if deadline {
					cancel()
					ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
				}
				defer cancel()
				done := make(chan error, 1)
				go func() {
					if streaming {
						s, err := client.SpeechToText.OpenStream(ctx, transcriptionRequest())
						if err == nil {
							defer s.Close()
							_, err = s.Next()
						}
						done <- err
					} else {
						_, err := client.SpeechToText.Transcribe(ctx, transcriptionRequest())
						done <- err
					}
				}()
				<-started
				if !deadline {
					cancel()
				}
				select {
				case err := <-done:
					expected := context.Canceled
					if deadline {
						expected = context.DeadlineExceeded
					}
					if !errors.Is(err, expected) {
						t.Fatalf("expected %v: %v", expected, err)
					}
				case <-time.After(3 * time.Second):
					t.Fatal("cancel did not interrupt")
				}
			})
		}
	}
}

func TestSpeechToTextCloseInterruptsNext(t *testing.T) {
	client := transcriptionClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	s, err := client.SpeechToText.OpenStream(context.Background(), transcriptionRequest())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := s.Next(); done <- err }()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected closed read error")
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not interrupt Next")
	}
}

func TestSpeechToTextRetryCancellation(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprint(streaming), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(429) }))
			defer server.Close()
			sleeping := make(chan struct{})
			client, err := NewClient(Config{BaseURL: server.URL, Retry: transport.RetryConfig{Sleep: func(ctx context.Context, _ time.Duration) error { close(sleeping); <-ctx.Done(); return ctx.Err() }}})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				var err error
				if streaming {
					_, err = client.SpeechToText.OpenStream(ctx, transcriptionRequest())
				} else {
					_, err = client.SpeechToText.Transcribe(ctx, transcriptionRequest())
				}
				done <- err
			}()
			<-sleeping
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("retry wait ignored cancellation")
			}
			if calls.Load() != 1 {
				t.Fatal("retried canceled request")
			}
			if _, err := client.SpeechToText.Transcribe(ctx, transcriptionRequest()); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if calls.Load() != 1 {
				t.Fatal("pre-canceled request reached network")
			}
		})
	}
}

func TestSpeechToTextStreamAutomaticallyCloses(t *testing.T) {
	for _, payload := range []string{`{"index":0,"delta":"","finish":true,"duration":1}`, `{"index":2,"delta":"","finish":false}`} {
		t.Run(payload, func(t *testing.T) {
			closed := make(chan struct{})
			client := transcriptionClient(t, func(w http.ResponseWriter, r *http.Request) {
				io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "data: %s\n\n", payload)
				w.(http.Flusher).Flush()
				<-r.Context().Done()
				close(closed)
			})
			s, err := client.SpeechToText.OpenStream(t.Context(), transcriptionRequest())
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			chunk, err := s.Next()
			if strings.Contains(payload, `"finish":true`) {
				if err != nil || !chunk.Finish {
					t.Fatalf("%+v %v", chunk, err)
				}
			} else if err == nil {
				t.Fatal("expected protocol error")
			}
			select {
			case <-closed:
			case <-time.After(time.Second):
				t.Fatal("stream body was not closed")
			}
		})
	}
}
