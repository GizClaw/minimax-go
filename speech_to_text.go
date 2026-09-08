package minimax

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/GizClaw/minimax-go/internal/protocol"
	"github.com/GizClaw/minimax-go/internal/transport"
)

// APIError is the SDK's normalized HTTP or business error; inspect with errors.As.
type APIError = protocol.APIError

// APIErrorDetail contains structured provider error fields.
type APIErrorDetail = protocol.ErrorDetail

// SpeechToTextModel selects a transcription model.
type SpeechToTextModel string

const SpeechToTextModelASR10 SpeechToTextModel = "asr-1.0"

// SpeechToTextFormat selects the synchronous result representation.
type SpeechToTextFormat string

const (
	SpeechToTextFormatJSON        SpeechToTextFormat = "json"
	SpeechToTextFormatVerboseJSON SpeechToTextFormat = "verbose_json"
	SpeechToTextFormatSRT         SpeechToTextFormat = "srt"
	SpeechToTextFormatVTT         SpeechToTextFormat = "vtt"
	// SpeechToTextMaxFileBytes is the documented 50 MiB audio upload limit.
	SpeechToTextMaxFileBytes = 50 * 1024 * 1024
)

// SpeechToTextFile contains containerized audio. Raw PCM is unsupported.
type SpeechToTextFile struct {
	// Name includes a supported extension: wav, aiff, flac, m4a (ALAC), mp3, aac, opus or ogg.
	Name string
	// Data is the audio content, at most 50 MiB and 500 seconds. Duration and codec are server-validated.
	// The caller must not modify Data until Transcribe or OpenStream returns.
	Data []byte
}

// SpeechToTextRequest supplies multipart fields and the language header.
// Transcribe sets stream=false; OpenStream sets stream=true.
type SpeechToTextRequest struct {
	// Model defaults to asr-1.0.
	Model SpeechToTextModel
	// File is the audio to recognize.
	File SpeechToTextFile
	// ResponseFormat defaults to json. OpenStream permits only json.
	ResponseFormat SpeechToTextFormat
	// Language is an optional BCP-47 hint sent as a header; empty enables mixed-language recognition.
	Language string
}

// SpeechToTextSegment describes one sentence in verbose JSON output.
type SpeechToTextSegment struct {
	// ID is the zero-based sentence index.
	ID int `json:"id"`
	// Start is the sentence start time in seconds.
	Start float64 `json:"start"`
	// End is the sentence end time in seconds.
	End float64 `json:"end"`
	// Speaker identifies a speaker, for example S1 or S2.
	Speaker string `json:"speaker"`
	// Text contains this sentence's recognized text.
	Text string `json:"text"`
}

// SpeechToTextResponse contains JSON recognition fields or verbatim subtitles.
type SpeechToTextResponse struct {
	// ResponseMeta contains status, response headers and request/trace identifiers.
	ResponseMeta ResponseMeta `json:"response_meta,omitzero"`
	// Text is the complete recognized text, including an empty transcript for silence.
	Text string `json:"text"`
	// Duration is the input duration in seconds used for billing; absent for subtitles.
	Duration *float64 `json:"duration,omitempty"`
	// NSpeakers is the speaker count, returned only for verbose_json.
	NSpeakers *int `json:"n_speakers,omitempty"`
	// Segments contains timestamped sentences, returned only for verbose_json.
	Segments []SpeechToTextSegment `json:"segments,omitempty"`
	// TraceID is the provider's body trace identifier, when returned.
	TraceID string `json:"trace_id,omitempty"`
	// Subtitles preserves the exact SRT or WebVTT response body.
	Subtitles string `json:"subtitles,omitempty"`
}

// SpeechToTextService implements POST /v1/speech_to_text.
type SpeechToTextService struct{ transport *transport.Client }

func (s *SpeechToTextService) prepare(request SpeechToTextRequest, streaming bool) (transport.UploadRequest, SpeechToTextFormat, error) {
	if s == nil || s.transport == nil {
		return transport.UploadRequest{}, "", errors.New("speech-to-text service is not initialized")
	}
	if request.Model == "" {
		request.Model = SpeechToTextModelASR10
	}
	if request.Model != SpeechToTextModelASR10 {
		return transport.UploadRequest{}, "", fmt.Errorf("unsupported transcription model %q", request.Model)
	}
	format := request.ResponseFormat
	if format == "" {
		format = SpeechToTextFormatJSON
	}
	switch format {
	case SpeechToTextFormatJSON, SpeechToTextFormatVerboseJSON, SpeechToTextFormatSRT, SpeechToTextFormatVTT:
	default:
		return transport.UploadRequest{}, "", fmt.Errorf("unsupported transcription response format %q", format)
	}
	if streaming && format != SpeechToTextFormatJSON {
		return transport.UploadRequest{}, "", errors.New("streaming transcription requires json response format")
	}
	if strings.TrimSpace(request.File.Name) == "" || strings.ContainsAny(request.File.Name, "\r\n") {
		return transport.UploadRequest{}, "", errors.New("transcription file requires a valid name")
	}
	switch strings.ToLower(filepath.Ext(request.File.Name)) {
	case ".wav", ".aiff", ".flac", ".m4a", ".mp3", ".aac", ".opus", ".ogg":
	default:
		return transport.UploadRequest{}, "", errors.New("transcription file has unsupported audio extension")
	}
	if len(request.File.Data) == 0 || len(request.File.Data) > SpeechToTextMaxFileBytes {
		return transport.UploadRequest{}, "", errors.New("transcription audio must contain 1 to 52428800 bytes")
	}
	if strings.ContainsAny(request.Language, "\r\n") {
		return transport.UploadRequest{}, "", errors.New("invalid transcription language header")
	}
	accept := "application/json"
	if streaming {
		accept = "text/event-stream"
	} else if format == SpeechToTextFormatSRT {
		accept = "text/plain"
	} else if format == SpeechToTextFormatVTT {
		accept = "text/vtt"
	}
	headers := make(http.Header)
	headers.Set("Accept", accept)
	headers.Set("language", request.Language)
	return transport.UploadRequest{Path: "/v1/speech_to_text", Headers: headers,
		FormFields: []transport.FormField{{Name: "model", Value: string(request.Model)}, {Name: "response_format", Value: string(format)}, {Name: "stream", Value: strconv.FormatBool(streaming)}},
		FileField:  "file", FileName: filepath.Base(request.File.Name), FileContentType: "application/octet-stream", FileData: request.File.Data}, format, nil
}

// Transcribe recognizes audio as JSON, verbose JSON, SRT or WebVTT.
func (s *SpeechToTextService) Transcribe(ctx context.Context, request SpeechToTextRequest) (*SpeechToTextResponse, error) {
	upload, format, err := s.prepare(request, false)
	if err != nil {
		return nil, err
	}
	raw, err := s.transport.OpenUploadWithMeta(ctx, upload, false)
	if err != nil {
		return nil, fmt.Errorf("transcribe audio: %w", err)
	}
	defer raw.Body.Close()
	contentType, _, err := mime.ParseMediaType(raw.Meta.Header.Get("Content-Type"))
	expected := upload.Headers.Get("Accept")
	if err != nil || contentType != expected {
		return nil, fmt.Errorf("transcription response content type %q, expected %q", contentType, expected)
	}
	body, err := io.ReadAll(raw.Body)
	if err != nil {
		return nil, fmt.Errorf("read transcription: %w", err)
	}
	response := &SpeechToTextResponse{}
	if format == SpeechToTextFormatSRT || format == SpeechToTextFormatVTT {
		response.Subtitles = string(body)
	} else {
		// Pointers distinguish missing fields from a valid empty transcript and zero duration.
		var wire struct {
			Text      *string               `json:"text"`
			Duration  *float64              `json:"duration"`
			NSpeakers *int                  `json:"n_speakers"`
			Segments  []SpeechToTextSegment `json:"segments"`
			TraceID   string                `json:"trace_id"`
		}
		if err := json.Unmarshal(body, &wire); err != nil {
			return nil, fmt.Errorf("decode transcription: %w", err)
		}
		if wire.Text == nil || wire.Duration == nil || *wire.Duration < 0 {
			return nil, errors.New("transcription response requires text and nonnegative duration")
		}
		if format == SpeechToTextFormatVerboseJSON && (wire.NSpeakers == nil || *wire.NSpeakers < 0 || wire.Segments == nil) {
			return nil, errors.New("verbose transcription requires speaker count and segments")
		}
		response.Text, response.Duration, response.NSpeakers, response.Segments, response.TraceID = *wire.Text, wire.Duration, wire.NSpeakers, wire.Segments, wire.TraceID
	}
	response.ResponseMeta = responseMetaFromTransport(raw.Meta)
	return response, nil
}
