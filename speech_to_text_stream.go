package minimax

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/GizClaw/minimax-go/internal/protocol"
	"github.com/GizClaw/minimax-go/internal/stream"
)

// SpeechToTextChunk is one incremental recognition event.
type SpeechToTextChunk struct {
	// Index is the sequential event index, starting at zero.
	Index int `json:"index"`
	// Delta is the newly recognized text; concatenate deltas in index order.
	Delta string `json:"delta"`
	// Finish marks the final event; Next subsequently returns io.EOF.
	Finish bool `json:"finish"`
	// Duration is the audio duration in seconds, present only on the final event.
	Duration *float64 `json:"duration,omitempty"`
}

// SpeechToTextStream reads ordered SSE events. Call Close when finished.
// Next must not be called concurrently with itself; Close may interrupt Next.
type SpeechToTextStream struct {
	// ResponseMeta contains the response headers and request/trace identifiers.
	ResponseMeta ResponseMeta
	body         io.ReadCloser
	reader       *stream.Reader
	nextIndex    int
	done         bool
	closed       atomic.Bool
	closeOnce    sync.Once
	closeErr     error
}

// OpenStream uploads audio and opens incremental JSON transcription over SSE.
// The supplied context controls both the upload and all subsequent reads.
func (s *SpeechToTextService) OpenStream(ctx context.Context, request SpeechToTextRequest) (*SpeechToTextStream, error) {
	upload, _, err := s.prepare(request, true)
	if err != nil {
		return nil, err
	}
	raw, err := s.transport.OpenUploadWithMeta(ctx, upload, true)
	if err != nil {
		return nil, fmt.Errorf("open transcription stream: %w", err)
	}
	return &SpeechToTextStream{ResponseMeta: responseMetaFromTransport(raw.Meta), body: raw.Body, reader: stream.NewReader(raw.Body)}, nil
}

// Next returns the next delta, including the finish event. EOF before finish is
// io.ErrUnexpectedEOF. Parse/read errors terminate the stream without replay.
func (s *SpeechToTextStream) Next() (*SpeechToTextChunk, error) {
	if s == nil || s.reader == nil {
		return nil, errors.New("transcription stream is not initialized")
	}
	if s.done || s.closed.Load() {
		return nil, io.EOF
	}
	for {
		event, err := s.reader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}
			return nil, s.fail(fmt.Errorf("read transcription stream: %w", err))
		}
		if event.Data == "" {
			continue
		}
		body := []byte(event.Data)
		if err := protocol.CheckResponseWithTrace(http.StatusOK, body, protocol.TraceMeta{RequestID: s.ResponseMeta.RequestID, TraceID: s.ResponseMeta.TraceID}); err != nil {
			return nil, s.fail(err)
		}
		var wire struct {
			Index    *int     `json:"index"`
			Delta    *string  `json:"delta"`
			Finish   *bool    `json:"finish"`
			Duration *float64 `json:"duration"`
		}
		if err := json.Unmarshal(body, &wire); err != nil {
			return nil, s.fail(fmt.Errorf("decode transcription event: %w", err))
		}
		if wire.Index == nil || wire.Delta == nil || wire.Finish == nil {
			return nil, s.fail(errors.New("transcription event requires index, delta and finish"))
		}
		if *wire.Index != s.nextIndex {
			return nil, s.fail(fmt.Errorf("transcription event index %d, expected %d", *wire.Index, s.nextIndex))
		}
		if *wire.Finish && (wire.Duration == nil || *wire.Duration < 0) {
			return nil, s.fail(errors.New("final transcription event requires nonnegative duration"))
		}
		if !*wire.Finish && wire.Duration != nil {
			return nil, s.fail(errors.New("transcription duration is only valid on the final event"))
		}
		s.nextIndex++
		chunk := &SpeechToTextChunk{Index: *wire.Index, Delta: *wire.Delta, Finish: *wire.Finish, Duration: wire.Duration}
		if chunk.Finish {
			s.done = true
			if err := s.Close(); err != nil {
				return nil, fmt.Errorf("close transcription stream: %w", err)
			}
		}
		return chunk, nil
	}
}

func (s *SpeechToTextStream) fail(err error) error {
	s.done = true
	return errors.Join(err, s.Close())
}

// Close releases the HTTP response body and interrupts pending reads. It is idempotent.
func (s *SpeechToTextStream) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		s.closed.Store(true)
		if s.body != nil {
			s.closeErr = s.body.Close()
		}
	})
	return s.closeErr
}
