package minimax

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GizClaw/minimax-go/internal/codec"
	"github.com/GizClaw/minimax-go/internal/protocol"
	"github.com/GizClaw/minimax-go/internal/stream"
	"github.com/GizClaw/minimax-go/internal/transport"
)

type SpeechStreamRequest struct {
	Model                string                   `json:"model,omitempty"`
	Text                 string                   `json:"text"`
	StreamOptions        *SpeechStreamOptions     `json:"stream_options,omitempty"`
	VoiceID              string                   `json:"voice_id,omitempty"`
	Speed                *float64                 `json:"speed,omitempty"`
	Vol                  *float64                 `json:"vol,omitempty"`
	Pitch                *int                     `json:"pitch,omitempty"`
	Emotion              string                   `json:"emotion,omitempty"`
	EnglishNormalization *bool                    `json:"english_normalization,omitempty"`
	LatexRead            *bool                    `json:"latex_read,omitempty"`
	OutputFormat         string                   `json:"output_format,omitempty"`
	AudioSetting         *SpeechAudioSetting      `json:"audio_setting,omitempty"`
	PronunciationDict    *SpeechPronunciationDict `json:"pronunciation_dict,omitempty"`
	TimberWeights        []SpeechTimberWeight     `json:"timbre_weights,omitempty"`
	LanguageBoost        string                   `json:"language_boost,omitempty"`
	VoiceModify          *SpeechVoiceModify       `json:"voice_modify,omitempty"`
	SubtitleEnable       *bool                    `json:"subtitle_enable,omitempty"`
	SubtitleType         string                   `json:"subtitle_type,omitempty"`
}

// SpeechStreamOptions controls the provider's terminal streaming response.
type SpeechStreamOptions struct {
	// ExcludeAggregatedAudio omits the final copy of all previously streamed
	// audio. Incremental audio and final usage metadata are still returned.
	ExcludeAggregatedAudio bool `json:"exclude_aggregated_audio"`
}

// SpeechChunk contains newly streamed audio and optional terminal metadata.
type SpeechChunk struct {
	Event       string
	Audio       []byte
	RawHexAudio string
	Done        bool
	// AggregatedAudio is the final complete copy when stream_options does not
	// exclude it. It is separate from Audio to avoid playing the stream twice.
	AggregatedAudio []byte
	// UsageCharacters is the provider's billed count from final extra_info.
	UsageCharacters *int64
}

// SpeechStream reads speech stream events and yields decoded audio chunks.
type SpeechStream struct {
	ResponseMeta ResponseMeta
	body         io.ReadCloser
	reader       *stream.Reader
	done         bool
	terminal     atomic.Bool
	cancel       context.CancelFunc
	closeOnce    sync.Once
	closeErr     error
}

type speechStreamWireRequest struct {
	Model             string                   `json:"model"`
	Text              string                   `json:"text"`
	Stream            bool                     `json:"stream"`
	StreamOptions     *SpeechStreamOptions     `json:"stream_options,omitempty"`
	OutputFormat      string                   `json:"output_format,omitempty"`
	VoiceSetting      *speechVoiceSetting      `json:"voice_setting,omitempty"`
	AudioSetting      *speechAudioSettingWire  `json:"audio_setting,omitempty"`
	PronunciationDict *SpeechPronunciationDict `json:"pronunciation_dict,omitempty"`
	TimberWeights     []SpeechTimberWeight     `json:"timbre_weights,omitempty"`
	LanguageBoost     string                   `json:"language_boost,omitempty"`
	VoiceModify       *SpeechVoiceModify       `json:"voice_modify,omitempty"`
	SubtitleEnable    *bool                    `json:"subtitle_enable,omitempty"`
	SubtitleType      string                   `json:"subtitle_type,omitempty"`
}

type speechStreamEventPayload struct {
	Data       speechStreamEventData  `json:"data"`
	AudioHex   string                 `json:"audio_hex,omitempty"`
	Audio      string                 `json:"audio,omitempty"`
	Hex        string                 `json:"hex,omitempty"`
	Chunk      string                 `json:"chunk,omitempty"`
	Output     string                 `json:"output,omitempty"`
	Type       string                 `json:"type,omitempty"`
	Status     speechStreamStatus     `json:"status,omitempty"`
	State      string                 `json:"state,omitempty"`
	Done       bool                   `json:"done,omitempty"`
	IsFinal    bool                   `json:"is_final,omitempty"`
	Finished   bool                   `json:"finished,omitempty"`
	StatusCode int                    `json:"status_code,omitempty"`
	StatusMsg  string                 `json:"status_msg,omitempty"`
	Error      string                 `json:"error,omitempty"`
	ErrorMsg   string                 `json:"error_msg,omitempty"`
	Message    string                 `json:"message,omitempty"`
	ExtraInfo  *speechStreamExtraInfo `json:"extra_info,omitempty"`
}

type speechStreamEventData struct {
	AudioHex   string             `json:"audio_hex,omitempty"`
	Audio      string             `json:"audio,omitempty"`
	Hex        string             `json:"hex,omitempty"`
	Chunk      string             `json:"chunk,omitempty"`
	Output     string             `json:"output,omitempty"`
	Type       string             `json:"type,omitempty"`
	Status     speechStreamStatus `json:"status,omitempty"`
	State      string             `json:"state,omitempty"`
	Done       bool               `json:"done,omitempty"`
	IsFinal    bool               `json:"is_final,omitempty"`
	Finished   bool               `json:"finished,omitempty"`
	StatusCode int                `json:"status_code,omitempty"`
	StatusMsg  string             `json:"status_msg,omitempty"`
	Error      string             `json:"error,omitempty"`
	ErrorMsg   string             `json:"error_msg,omitempty"`
	Message    string             `json:"message,omitempty"`
}

type speechStreamExtraInfo struct {
	UsageCharacters *int64 `json:"usage_characters,omitempty"`
}

// Official T2A responses use numeric status 1/2. Some compatible endpoints
// use text states, so preserve both representations without weakening errors.
type speechStreamStatus string

func (s *speechStreamStatus) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = ""
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*s = speechStreamStatus(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("decode speech stream status: %w", err)
	}
	if _, err := number.Int64(); err != nil {
		return fmt.Errorf("decode speech stream integer status: %w", err)
	}
	*s = speechStreamStatus(number.String())
	return nil
}

// OpenStream opens a speech synthesis stream and returns a stream reader.
func (s *SpeechService) OpenStream(ctx context.Context, request SpeechStreamRequest) (*SpeechStream, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("speech service is not initialized")
	}

	request.Text = strings.TrimSpace(request.Text)
	request.Model = strings.TrimSpace(request.Model)
	request.VoiceID = strings.TrimSpace(request.VoiceID)
	request.OutputFormat = strings.TrimSpace(request.OutputFormat)
	request.Emotion = strings.TrimSpace(request.Emotion)
	request.LanguageBoost = strings.TrimSpace(request.LanguageBoost)
	request.SubtitleType = strings.TrimSpace(request.SubtitleType)

	if request.Text == "" {
		return nil, errors.New("speech stream request text is empty")
	}

	if request.Model == "" {
		request.Model = defaultSpeechModel
	}

	if request.OutputFormat == "" {
		request.OutputFormat = defaultSpeechOutputFormat
	}

	if !strings.EqualFold(request.OutputFormat, defaultSpeechOutputFormat) {
		return nil, fmt.Errorf(
			"speech stream output format %q is not supported, only %q is supported",
			request.OutputFormat,
			defaultSpeechOutputFormat,
		)
	}
	request.OutputFormat = defaultSpeechOutputFormat

	wireReq := speechStreamWireRequest{
		Model:             request.Model,
		Text:              request.Text,
		Stream:            true,
		StreamOptions:     request.StreamOptions,
		OutputFormat:      request.OutputFormat,
		AudioSetting:      speechAudioSettingForHTTP(request.AudioSetting),
		PronunciationDict: request.PronunciationDict,
		TimberWeights:     trimSpeechTimberWeights(request.TimberWeights),
		LanguageBoost:     request.LanguageBoost,
		VoiceModify:       request.VoiceModify,
		SubtitleEnable:    request.SubtitleEnable,
		SubtitleType:      request.SubtitleType,
	}

	if request.VoiceID != "" || request.Speed != nil || request.Vol != nil || request.Pitch != nil ||
		request.Emotion != "" || request.EnglishNormalization != nil || request.LatexRead != nil {
		wireReq.VoiceSetting = &speechVoiceSetting{
			VoiceID:              request.VoiceID,
			Speed:                request.Speed,
			Vol:                  request.Vol,
			Pitch:                request.Pitch,
			Emotion:              request.Emotion,
			EnglishNormalization: request.EnglishNormalization,
			LatexRead:            request.LatexRead,
		}
	}

	streamCtx, cancel := context.WithCancel(ctx)
	streamResponse, err := s.transport.OpenStreamWithMeta(streamCtx, transport.StreamRequest{
		Method: http.MethodPost,
		Path:   s.streamPath(),
		Body:   wireReq,
	})
	if err != nil {
		cancel()
		return nil, err
	}

	return &SpeechStream{
		ResponseMeta: responseMetaFromTransport(streamResponse.Meta),
		body:         streamResponse.Body,
		reader:       stream.NewReader(streamResponse.Body),
		cancel:       cancel,
	}, nil
}

// Stream is an alias of OpenStream.
func (s *SpeechService) Stream(ctx context.Context, request SpeechStreamRequest) (*SpeechStream, error) {
	return s.OpenStream(ctx, request)
}

// Next returns the next speech chunk; returns io.EOF when stream completes.
func (s *SpeechStream) Next() (*SpeechChunk, error) {
	if s == nil {
		return nil, errors.New("speech stream is nil")
	}

	if s.reader == nil {
		return nil, errors.New("speech stream reader is not initialized")
	}

	if s.done {
		return nil, io.EOF
	}

	for {
		event, err := s.reader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				s.done = true
			}
			return nil, err
		}

		chunk, parseErr := decodeSpeechStreamEvent(event)
		if parseErr != nil {
			return nil, parseErr
		}

		if chunk == nil {
			continue
		}

		if chunk.Done {
			s.done = true
			s.terminal.Store(true)
		}

		return chunk, nil
	}
}

// Close releases the stream. After a terminal event, it drains at most 64 KiB
// for up to 100ms so normal chunked HTTP responses can return to the pool.
// Abort paths close immediately; the remaining audio is never replayed.
func (s *SpeechStream) Close() error {
	if s == nil || s.body == nil {
		return nil
	}

	s.closeOnce.Do(func() {
		if s.cancel != nil {
			defer s.cancel()
		}
		if s.terminal.Load() && s.cancel != nil {
			finished := make(chan struct{})
			timer := time.AfterFunc(100*time.Millisecond, func() {
				defer close(finished)
				s.cancel()
				_ = s.body.Close()
			})
			_, _ = io.CopyN(io.Discard, s.body, 64<<10)
			if !timer.Stop() {
				<-finished
			}
		} else if s.cancel != nil {
			s.cancel()
		}
		s.closeErr = s.body.Close()
	})

	return s.closeErr
}

func decodeSpeechStreamEvent(event stream.Event) (*SpeechChunk, error) {
	eventName := strings.TrimSpace(event.Event)
	rawData := strings.TrimSpace(event.Data)

	if rawData == "" {
		if isSpeechStreamDoneEvent(eventName) {
			return &SpeechChunk{Event: eventName, Done: true}, nil
		}
		return nil, nil
	}

	if isSpeechStreamDoneData(rawData) {
		return &SpeechChunk{Event: eventName, Done: true}, nil
	}

	body := []byte(rawData)
	if baseResp, ok := protocol.ParseBaseResp(body); ok && baseResp.StatusCode != 0 {
		return nil, protocol.NewBaseRespError(http.StatusOK, baseResp, body)
	}

	var payload speechStreamEventPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		if isSpeechStreamDoneEvent(eventName) {
			return &SpeechChunk{Event: eventName, Done: true}, nil
		}
		return nil, fmt.Errorf("decode speech stream event payload: %w", err)
	}

	statusCode, statusMsg := payload.status()
	if statusCode != 0 {
		return nil, protocol.NewBaseRespError(http.StatusOK, protocol.BaseResp{
			StatusCode: statusCode,
			StatusMsg:  statusMsg,
		}, body)
	}

	isDone := payload.done() || isSpeechStreamDoneEvent(eventName)
	chunk := &SpeechChunk{Event: eventName, Done: isDone}
	if payload.ExtraInfo != nil {
		chunk.UsageCharacters = cloneInt64Pointer(payload.ExtraInfo.UsageCharacters)
	}

	hexAudio := payload.hexAudio()
	if hexAudio == "" {
		if isDone {
			return chunk, nil
		}

		if strings.EqualFold(eventName, "error") {
			msg := payload.errorMessage()
			if msg == "" {
				msg = rawData
			}
			return nil, fmt.Errorf("speech stream server error: %s", msg)
		}
		return nil, nil
	}

	audio, err := codec.DecodeHexAudio(hexAudio)
	if err != nil {
		return nil, fmt.Errorf("decode speech stream audio chunk: %w", err)
	}

	chunk.RawHexAudio = hexAudio
	if payload.Data.Status == "2" || payload.Status == "2" {
		chunk.AggregatedAudio = audio
	} else {
		chunk.Audio = audio
	}
	return chunk, nil
}

func (s *SpeechService) streamPath() string {
	if s == nil {
		return defaultSpeechStreamPath
	}

	if endpoint := strings.TrimSpace(s.streamEndpoint); endpoint != "" {
		return endpoint
	}

	if endpoint := strings.TrimSpace(s.endpoint); endpoint != "" {
		return endpoint
	}

	return defaultSpeechStreamPath
}

func (p speechStreamEventPayload) hexAudio() string {
	return firstNonEmpty(
		strings.TrimSpace(p.Data.AudioHex),
		strings.TrimSpace(p.Data.Audio),
		strings.TrimSpace(p.Data.Hex),
		strings.TrimSpace(p.Data.Chunk),
		strings.TrimSpace(p.Data.Output),
		strings.TrimSpace(p.AudioHex),
		strings.TrimSpace(p.Audio),
		strings.TrimSpace(p.Hex),
		strings.TrimSpace(p.Chunk),
		strings.TrimSpace(p.Output),
	)
}

func (p speechStreamEventPayload) done() bool {
	if p.Done || p.IsFinal || p.Finished || p.Data.Done || p.Data.IsFinal || p.Data.Finished {
		return true
	}

	statuses := []string{
		p.Type,
		string(p.Status),
		p.State,
		p.Data.Type,
		string(p.Data.Status),
		p.Data.State,
	}

	return slices.ContainsFunc(statuses, isSpeechStreamDoneStatus)
}

func (p speechStreamEventPayload) status() (int, string) {
	statusCode := p.StatusCode
	if statusCode == 0 {
		statusCode = p.Data.StatusCode
	}

	statusMsg := firstNonEmpty(
		strings.TrimSpace(p.StatusMsg),
		strings.TrimSpace(p.Data.StatusMsg),
		p.errorMessage(),
	)

	return statusCode, statusMsg
}

func (p speechStreamEventPayload) errorMessage() string {
	return firstNonEmpty(
		strings.TrimSpace(p.ErrorMsg),
		strings.TrimSpace(p.Data.ErrorMsg),
		strings.TrimSpace(p.Error),
		strings.TrimSpace(p.Data.Error),
		strings.TrimSpace(p.Message),
		strings.TrimSpace(p.Data.Message),
	)
}

func isSpeechStreamDoneEvent(eventName string) bool {
	switch strings.ToLower(strings.TrimSpace(eventName)) {
	case "done", "finish", "finished", "complete", "completed", "end", "ended":
		return true
	default:
		return false
	}
}

func isSpeechStreamDoneStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "2", "done", "finish", "finished", "complete", "completed", "end", "ended":
		return true
	default:
		return false
	}
}

func isSpeechStreamDoneData(data string) bool {
	normalized := strings.TrimSpace(data)
	normalized = strings.TrimPrefix(normalized, `"`)
	normalized = strings.TrimSuffix(normalized, `"`)

	return strings.EqualFold(normalized, "[DONE]") || strings.EqualFold(normalized, "DONE")
}
