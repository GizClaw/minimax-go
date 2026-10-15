package minimax

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/GizClaw/minimax-go/internal/codec"
	"github.com/GizClaw/minimax-go/internal/transport"
	"github.com/coder/websocket"
)

const defaultSpeechBidiPath = "/ws/v1/t2a_v2_bidi"

// SpeechBidiRequest configures one streaming synthesis session. Send text after
// OpenBidi returns; the provider owns sentence buffering and segmentation.
type SpeechBidiRequest struct {
	Model                string
	VoiceID              string
	SessionID            string
	Speed                *float64
	Vol                  *float64
	Pitch                *int
	Emotion              string
	EnglishNormalization *bool
	LatexRead            *bool
	AudioSetting         *SpeechAudioSetting
	PronunciationDict    *SpeechPronunciationDict
	TimberWeights        []SpeechTimberWeight
	LanguageBoost        string
	VoiceModify          *SpeechVoiceModify
	SubtitleEnable       *bool
	SubtitleType         string
	ContinuousSound      *bool
}

// SpeechBidiEvent is a provider event. IsFinal ends one audio request;
// sentence_end ends a sentence; only task_finished ends the session.
type SpeechBidiEvent struct {
	Event           string
	SessionID       string
	ConnectID       string
	TraceID         string
	Audio           []byte
	IsFinal         bool
	UsageCharacters *int64
	Done            bool
}

// SpeechBidi is one full-duplex speech session. One goroutine may call Next
// while others send text/control events. Writes are serialized in call
// acquisition order; callers must order Flush/Cancel/Finish after their text.
// Read the task_flushed/task_canceled acknowledgement before starting the
// next turn. Call Close to release the connection on every exit path.
type SpeechBidi struct {
	conn      *websocket.Conn
	writeGate chan struct{}
	closed    chan struct{}
	closeOnce sync.Once
	closeErr  error
	finishing atomic.Bool
	done      atomic.Bool
}

// OpenBidi connects and starts a bidirectional TTS session without sending
// text or finishing it. The context controls connection/startup IO; pass an
// invocation context to subsequent methods and close the session on cancel.
func (s *SpeechService) OpenBidi(ctx context.Context, request SpeechBidiRequest) (*SpeechBidi, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("speech service is not initialized")
	}
	start, err := buildSpeechWebSocketStartMessage(SpeechWebSocketRequest{
		Model: request.Model, VoiceID: request.VoiceID, Speed: request.Speed,
		Vol: request.Vol, Pitch: request.Pitch, Emotion: request.Emotion,
		EnglishNormalization: request.EnglishNormalization, LatexRead: request.LatexRead,
		AudioSetting: request.AudioSetting, PronunciationDict: request.PronunciationDict,
		TimberWeights: request.TimberWeights, LanguageBoost: request.LanguageBoost,
		VoiceModify: request.VoiceModify, SubtitleEnable: request.SubtitleEnable,
		SubtitleType: request.SubtitleType, ContinuousSound: request.ContinuousSound,
	})
	if err != nil {
		return nil, fmt.Errorf("speech bidi configuration: %w", err)
	}
	dial, err := s.transport.BuildWebSocketDialConfig(ctx, transport.WebSocketRequest{Path: defaultSpeechBidiPath})
	if err != nil {
		return nil, err
	}
	conn, _, err := websocket.Dial(ctx, dial.URL, &websocket.DialOptions{HTTPHeader: dial.Header, HTTPClient: dial.HTTPClient})
	if err != nil {
		return nil, fmt.Errorf("speech bidi dial: %w", err)
	}
	conn.SetReadLimit(16 << 20)
	bidi := &SpeechBidi{conn: conn, writeGate: make(chan struct{}, 1), closed: make(chan struct{})}
	bidi.writeGate <- struct{}{}
	if err := bidi.waitFor(ctx, "connected_success"); err != nil {
		_ = bidi.Close()
		return nil, err
	}
	payload := struct {
		speechWebSocketStartMessage
		SessionID string `json:"session_id,omitempty"`
	}{speechWebSocketStartMessage: start, SessionID: strings.TrimSpace(request.SessionID)}
	if err := bidi.write(ctx, payload, false); err != nil {
		_ = bidi.Close()
		return nil, err
	}
	if err := bidi.waitFor(ctx, "task_started"); err != nil {
		_ = bidi.Close()
		return nil, err
	}
	return bidi, nil
}

// SendText appends UTF-8 text without trimming or adding punctuation.
// The provider buffers it into sentences and discards whitespace-only
// messages; keep spaces/newlines with adjacent text when chunking.
// A message is limited to 10,000
// Unicode characters; larger inputs must be chunked by the caller.
func (s *SpeechBidi) SendText(ctx context.Context, text string) error {
	if !utf8.ValidString(text) {
		return errors.New("speech bidi text is not valid UTF-8")
	}
	if utf8.RuneCountInString(text) > 10000 {
		return errors.New("speech bidi text exceeds 10000 characters")
	}
	return s.write(ctx, speechWebSocketContinueMessage{Event: "task_continue", Text: text}, false)
}

// Flush requests synthesis of pending text without closing the session.
// Keep calling Next until task_flushed to observe completion of this turn.
func (s *SpeechBidi) Flush(ctx context.Context) error {
	return s.write(ctx, speechWebSocketFinishMessage{Event: "task_flush"}, false)
}

// Cancel discards pending text and interrupts synthesis while retaining the
// session. Audio received before task_canceled can belong to the old turn.
func (s *SpeechBidi) Cancel(ctx context.Context) error {
	return s.write(ctx, speechWebSocketFinishMessage{Event: "task_cancel"}, false)
}

// Finish ends text input. Next must continue until task_finished, including
// any audio synthesized from the remaining buffer. Repeated Finish is safe.
func (s *SpeechBidi) Finish(ctx context.Context) error {
	return s.write(ctx, speechWebSocketFinishMessage{Event: "task_finish"}, true)
}

// Ping keeps an idle session active. It requires a concurrently running Next
// reader to process the pong. The provider closes idle sessions after 120s.
func (s *SpeechBidi) Ping(ctx context.Context) error {
	if s == nil || s.conn == nil {
		return io.ErrClosedPipe
	}
	return s.conn.Ping(ctx)
}

func (s *SpeechBidi) write(ctx context.Context, value any, finish bool) error {
	if s == nil || s.conn == nil {
		return io.ErrClosedPipe
	}
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-s.closed:
		return io.ErrClosedPipe
	case <-s.writeGate:
	}
	defer func() { s.writeGate <- struct{}{} }()
	if err := ctx.Err(); err != nil {
		return context.Cause(ctx)
	}
	if s.finishing.Load() || s.done.Load() {
		if finish {
			return nil
		}
		return io.ErrClosedPipe
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal speech bidi event: %w", err)
	}
	if err := s.conn.Write(ctx, websocket.MessageText, payload); err != nil {
		return fmt.Errorf("write speech bidi event: %w", err)
	}
	if finish {
		s.finishing.Store(true)
	}
	return nil
}

// Next reads an audio, sentence, or control event. It never treats IsFinal,
// sentence_end, task_flushed, or task_canceled as session completion.
// Provider errors 2204/2205 are returned without closing the session; callers
// may recover using their request ordering. IO errors are never retried.
func (s *SpeechBidi) Next(ctx context.Context) (*SpeechBidiEvent, error) {
	if s == nil || s.conn == nil {
		return nil, io.ErrClosedPipe
	}
	if s.done.Load() {
		return nil, io.EOF
	}
	kind, raw, err := s.conn.Read(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, context.Cause(ctx)
		}
		if status := websocket.CloseStatus(err); status == websocket.StatusNormalClosure || status == websocket.StatusGoingAway {
			return nil, fmt.Errorf("speech bidi closed before task_finished: %w", io.ErrUnexpectedEOF)
		}
		return nil, fmt.Errorf("read speech bidi event: %w", err)
	}
	if kind != websocket.MessageText {
		return nil, errors.New("speech bidi received a binary event")
	}
	message, err := parseSpeechWebSocketRawMessage(raw)
	if err != nil {
		return nil, err
	}
	if err := message.apiError(raw); err != nil {
		return nil, err
	}
	if strings.EqualFold(message.Event, "task_failed") {
		return nil, message.failureError(raw)
	}
	var audio []byte
	if hex := message.hexAudio(); hex != "" {
		audio, err = codec.DecodeHexAudio(hex)
		if err != nil {
			return nil, fmt.Errorf("decode speech bidi audio: %w", err)
		}
	}
	event := &SpeechBidiEvent{
		Event: strings.TrimSpace(message.Event), SessionID: message.SessionID,
		TraceID: message.TraceID, Audio: audio, IsFinal: message.IsFinal,
		Done: message.Event == "task_finished",
	}
	if message.ExtraInfo != nil {
		event.UsageCharacters = cloneInt64Pointer(message.ExtraInfo.UsageCharacters)
	}
	if value, ok := message.Raw["connect_id"]; ok {
		if err := json.Unmarshal(value, &event.ConnectID); err != nil {
			return nil, err
		}
	}
	if event.Done {
		s.done.Store(true)
	}
	return event, nil
}

func (s *SpeechBidi) waitFor(ctx context.Context, expected string) error {
	event, err := s.Next(ctx)
	if err != nil {
		return err
	}
	if event.Event != expected {
		return fmt.Errorf("speech bidi startup event %q, want %q", event.Event, expected)
	}
	return nil
}

// Close immediately releases IO and unblocks pending reads/writes. Use Finish
// and read task_finished first when the remaining audio must be preserved.
func (s *SpeechBidi) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		close(s.closed)
		s.closeErr = s.conn.CloseNow()
	})
	return s.closeErr
}
