package minimax

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/GizClaw/minimax-go/internal/codec"
	"github.com/GizClaw/minimax-go/internal/protocol"
	"github.com/GizClaw/minimax-go/internal/transport"
	"github.com/coder/websocket"
)

const defaultSpeechWebSocketPath = "/ws/v1/t2a_v2"

type SpeechWebSocketRequest struct {
	Model                string                   `json:"model,omitempty"`
	Text                 string                   `json:"text"`
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
	ContinuousSound      *bool                    `json:"continuous_sound,omitempty"`
}

type SpeechWebSocket struct {
	conn      *websocket.Conn
	closeOnce sync.Once
	closeErr  error
	done      bool
}

type SpeechWebSocketEvent struct {
	Event       string          `json:"event,omitempty"`
	SessionID   string          `json:"session_id,omitempty"`
	TraceID     string          `json:"trace_id,omitempty"`
	Audio       []byte          `json:"audio,omitempty"`
	RawHexAudio string          `json:"raw_hex_audio,omitempty"`
	Done        bool            `json:"done,omitempty"`
	Raw         json.RawMessage `json:"-"`
}

type speechWebSocketStartMessage struct {
	Event             string                   `json:"event"`
	Model             string                   `json:"model"`
	VoiceSetting      *speechVoiceSetting      `json:"voice_setting,omitempty"`
	AudioSetting      *speechAudioSettingWire  `json:"audio_setting,omitempty"`
	PronunciationDict *SpeechPronunciationDict `json:"pronunciation_dict,omitempty"`
	TimberWeights     []SpeechTimberWeight     `json:"timbre_weights,omitempty"`
	LanguageBoost     string                   `json:"language_boost,omitempty"`
	VoiceModify       *SpeechVoiceModify       `json:"voice_modify,omitempty"`
	SubtitleEnable    *bool                    `json:"subtitle_enable,omitempty"`
	SubtitleType      string                   `json:"subtitle_type,omitempty"`
	ContinuousSound   *bool                    `json:"continuous_sound,omitempty"`
}

type speechWebSocketContinueMessage struct {
	Event string `json:"event"`
	Text  string `json:"text"`
}

type speechWebSocketFinishMessage struct {
	Event string `json:"event"`
}

type speechWebSocketRawMessage struct {
	SessionID  string                     `json:"session_id,omitempty"`
	Event      string                     `json:"event,omitempty"`
	TraceID    string                     `json:"trace_id,omitempty"`
	Data       speechWebSocketRawData     `json:"data"`
	ExtraInfo  *speechTaskMetaRaw         `json:"extra_info,omitempty"`
	BaseResp   *protocol.BaseResp         `json:"base_resp,omitempty"`
	StatusCode int                        `json:"status_code,omitempty"`
	StatusMsg  string                     `json:"status_msg,omitempty"`
	Error      string                     `json:"error,omitempty"`
	ErrorMsg   string                     `json:"error_msg,omitempty"`
	Message    string                     `json:"message,omitempty"`
	Raw        map[string]json.RawMessage `json:"-"`
}

type speechWebSocketRawData struct {
	AudioHex string `json:"audio_hex,omitempty"`
	Audio    string `json:"audio,omitempty"`
	Hex      string `json:"hex,omitempty"`
	Chunk    string `json:"chunk,omitempty"`
	Output   string `json:"output,omitempty"`
}

// OpenWebSocket opens the official MiniMax T2A WebSocket protocol path.
func (s *SpeechService) OpenWebSocket(ctx context.Context, request SpeechWebSocketRequest) (*SpeechWebSocket, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("speech service is not initialized")
	}

	start, continueMessage, err := buildSpeechWebSocketMessages(request)
	if err != nil {
		return nil, err
	}

	dialConfig, err := s.transport.BuildWebSocketDialConfig(ctx, transport.WebSocketRequest{
		Path: s.webSocketPath(),
	})
	if err != nil {
		return nil, err
	}

	conn, _, err := websocket.Dial(ctx, dialConfig.URL, &websocket.DialOptions{
		HTTPHeader: dialConfig.Header,
	})
	if err != nil {
		return nil, fmt.Errorf("speech websocket dial: %w", err)
	}

	ws := &SpeechWebSocket{conn: conn}
	if err := ws.waitForEvent(ctx, "connected_success"); err != nil {
		_ = ws.Close()
		return nil, err
	}
	if err := ws.writeJSON(ctx, start); err != nil {
		_ = ws.Close()
		return nil, err
	}
	if err := ws.waitForEvent(ctx, "task_started"); err != nil {
		_ = ws.Close()
		return nil, err
	}
	if err := ws.writeJSON(ctx, continueMessage); err != nil {
		_ = ws.Close()
		return nil, err
	}
	if err := ws.writeJSON(ctx, speechWebSocketFinishMessage{Event: "task_finish"}); err != nil {
		_ = ws.Close()
		return nil, err
	}

	return ws, nil
}

// Next reads the next WebSocket audio or terminal event.
func (s *SpeechWebSocket) Next(ctx context.Context) (*SpeechWebSocketEvent, error) {
	if s == nil || s.conn == nil {
		return nil, errors.New("speech websocket is not initialized")
	}
	if s.done {
		return nil, io.EOF
	}

	for {
		raw, err := s.readText(ctx)
		if err != nil {
			return nil, err
		}

		event, err := decodeSpeechWebSocketEvent(raw)
		if err != nil {
			return nil, err
		}
		if event == nil {
			continue
		}
		if event.Done {
			s.done = true
		}
		return event, nil
	}
}

// Close closes the WebSocket connection.
func (s *SpeechWebSocket) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}

	s.closeOnce.Do(func() {
		s.closeErr = s.conn.Close(websocket.StatusNormalClosure, "")
	})
	return s.closeErr
}

func (s *SpeechWebSocket) waitForEvent(ctx context.Context, want string) error {
	for {
		raw, err := s.readText(ctx)
		if err != nil {
			return err
		}

		message, err := parseSpeechWebSocketRawMessage(raw)
		if err != nil {
			return err
		}
		if err := message.apiError(raw); err != nil {
			return err
		}

		event := strings.TrimSpace(message.Event)
		if strings.EqualFold(event, want) {
			return nil
		}
		if strings.EqualFold(event, "task_failed") {
			return message.failureError(raw)
		}
	}
}

func (s *SpeechWebSocket) writeJSON(ctx context.Context, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal speech websocket message: %w", err)
	}
	if err := s.conn.Write(ctx, websocket.MessageText, body); err != nil {
		return fmt.Errorf("write speech websocket message: %w", err)
	}
	return nil
}

func (s *SpeechWebSocket) readText(ctx context.Context) ([]byte, error) {
	messageType, body, err := s.conn.Read(ctx)
	if err != nil {
		switch websocket.CloseStatus(err) {
		case websocket.StatusNormalClosure, websocket.StatusGoingAway:
			return nil, io.EOF
		}
		return nil, err
	}
	if messageType != websocket.MessageText {
		return nil, fmt.Errorf("speech websocket unexpected binary frame: message_type=%v", messageType)
	}
	return body, nil
}

func buildSpeechWebSocketMessages(request SpeechWebSocketRequest) (speechWebSocketStartMessage, speechWebSocketContinueMessage, error) {
	request.Model = strings.TrimSpace(request.Model)
	request.Text = strings.TrimSpace(request.Text)
	request.VoiceID = strings.TrimSpace(request.VoiceID)
	request.Emotion = strings.TrimSpace(request.Emotion)
	request.OutputFormat = strings.TrimSpace(request.OutputFormat)
	request.LanguageBoost = strings.TrimSpace(request.LanguageBoost)
	request.SubtitleType = strings.TrimSpace(request.SubtitleType)

	if request.Text == "" {
		return speechWebSocketStartMessage{}, speechWebSocketContinueMessage{}, errors.New("speech websocket request text is empty")
	}
	if request.Model == "" {
		request.Model = defaultSpeechModel
	}
	if request.OutputFormat != "" && !strings.EqualFold(request.OutputFormat, defaultSpeechOutputFormat) {
		return speechWebSocketStartMessage{}, speechWebSocketContinueMessage{}, fmt.Errorf(
			"speech websocket output format %q is not supported, only %q is supported",
			request.OutputFormat,
			defaultSpeechOutputFormat,
		)
	}

	timberWeights := trimSpeechTimberWeights(request.TimberWeights)
	if request.VoiceID == "" && len(timberWeights) == 0 {
		return speechWebSocketStartMessage{}, speechWebSocketContinueMessage{}, errors.New("speech websocket request voice_id is empty")
	}

	start := speechWebSocketStartMessage{
		Event:             "task_start",
		Model:             request.Model,
		AudioSetting:      speechAudioSettingForHTTP(request.AudioSetting),
		PronunciationDict: request.PronunciationDict,
		TimberWeights:     timberWeights,
		LanguageBoost:     request.LanguageBoost,
		VoiceModify:       request.VoiceModify,
		SubtitleEnable:    request.SubtitleEnable,
		SubtitleType:      request.SubtitleType,
		ContinuousSound:   request.ContinuousSound,
	}

	if request.VoiceID != "" || request.Speed != nil || request.Vol != nil || request.Pitch != nil ||
		request.Emotion != "" || request.EnglishNormalization != nil || request.LatexRead != nil {
		start.VoiceSetting = &speechVoiceSetting{
			VoiceID:              request.VoiceID,
			Speed:                request.Speed,
			Vol:                  request.Vol,
			Pitch:                request.Pitch,
			Emotion:              request.Emotion,
			EnglishNormalization: request.EnglishNormalization,
			LatexRead:            request.LatexRead,
		}
	}

	return start, speechWebSocketContinueMessage{
		Event: "task_continue",
		Text:  request.Text,
	}, nil
}

func decodeSpeechWebSocketEvent(raw []byte) (*SpeechWebSocketEvent, error) {
	message, err := parseSpeechWebSocketRawMessage(raw)
	if err != nil {
		return nil, err
	}
	if err := message.apiError(raw); err != nil {
		return nil, err
	}

	eventName := strings.TrimSpace(message.Event)
	if strings.EqualFold(eventName, "task_failed") {
		return nil, message.failureError(raw)
	}
	if isSpeechWebSocketDoneEvent(eventName) {
		return &SpeechWebSocketEvent{
			Event:     eventName,
			SessionID: strings.TrimSpace(message.SessionID),
			TraceID:   strings.TrimSpace(message.TraceID),
			Done:      true,
			Raw:       cloneRawBytes(raw),
		}, nil
	}

	hexAudio := message.hexAudio()
	if hexAudio == "" {
		return nil, nil
	}

	audio, err := codec.DecodeHexAudio(hexAudio)
	if err != nil {
		return nil, fmt.Errorf("decode speech websocket audio chunk: %w", err)
	}

	return &SpeechWebSocketEvent{
		Event:       eventName,
		SessionID:   strings.TrimSpace(message.SessionID),
		TraceID:     strings.TrimSpace(message.TraceID),
		Audio:       audio,
		RawHexAudio: hexAudio,
		Raw:         cloneRawBytes(raw),
	}, nil
}

func parseSpeechWebSocketRawMessage(raw []byte) (speechWebSocketRawMessage, error) {
	var message speechWebSocketRawMessage
	if err := json.Unmarshal(raw, &message); err != nil {
		return speechWebSocketRawMessage{}, fmt.Errorf("decode speech websocket message: %w", err)
	}
	return message, nil
}

func (m speechWebSocketRawMessage) hexAudio() string {
	return firstNonEmpty(
		strings.TrimSpace(m.Data.AudioHex),
		strings.TrimSpace(m.Data.Audio),
		strings.TrimSpace(m.Data.Hex),
		strings.TrimSpace(m.Data.Chunk),
		strings.TrimSpace(m.Data.Output),
	)
}

func (m speechWebSocketRawMessage) apiError(raw []byte) error {
	if m.BaseResp != nil && m.BaseResp.StatusCode != 0 {
		return protocol.NewBaseRespError(http.StatusOK, *m.BaseResp, raw)
	}
	if m.StatusCode != 0 {
		return protocol.NewBaseRespError(http.StatusOK, protocol.BaseResp{
			StatusCode: m.StatusCode,
			StatusMsg:  firstNonEmpty(strings.TrimSpace(m.StatusMsg), m.errorMessage()),
		}, raw)
	}
	return nil
}

func (m speechWebSocketRawMessage) failureError(raw []byte) error {
	if err := m.apiError(raw); err != nil {
		return err
	}

	return protocol.NewBaseRespError(http.StatusOK, protocol.BaseResp{
		StatusCode: -1,
		StatusMsg:  firstNonEmpty(m.errorMessage(), "speech websocket task failed"),
	}, raw)
}

func (m speechWebSocketRawMessage) errorMessage() string {
	return firstNonEmpty(
		strings.TrimSpace(m.ErrorMsg),
		strings.TrimSpace(m.Error),
		strings.TrimSpace(m.Message),
		strings.TrimSpace(m.StatusMsg),
	)
}

func isSpeechWebSocketDoneEvent(eventName string) bool {
	switch strings.ToLower(strings.TrimSpace(eventName)) {
	case "task_finished", "task_finish", "done", "finished", "finish", "completed", "complete", "end", "ended":
		return true
	default:
		return false
	}
}

func (s *SpeechService) webSocketPath() string {
	if s == nil {
		return defaultSpeechWebSocketPath
	}
	if path := strings.TrimSpace(s.websocketPath); path != "" {
		return path
	}
	return defaultSpeechWebSocketPath
}

func cloneRawBytes(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	copied := make([]byte, len(raw))
	copy(copied, raw)
	return copied
}

func (r *speechWebSocketRawMessage) UnmarshalJSON(data []byte) error {
	type alias speechWebSocketRawMessage

	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	delete(raw, "session_id")
	delete(raw, "event")
	delete(raw, "trace_id")
	delete(raw, "data")
	delete(raw, "extra_info")
	delete(raw, "base_resp")
	delete(raw, "status_code")
	delete(raw, "status_msg")
	delete(raw, "error")
	delete(raw, "error_msg")
	delete(raw, "message")

	*r = speechWebSocketRawMessage(parsed)
	if len(raw) > 0 {
		r.Raw = raw
	} else {
		r.Raw = nil
	}
	return nil
}
