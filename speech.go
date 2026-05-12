package minimax

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/GizClaw/minimax-go/internal/codec"
	"github.com/GizClaw/minimax-go/internal/transport"
)

const (
	defaultSpeechSynthesizePath = "/v1/t2a_v2"
	defaultSpeechStreamPath     = defaultSpeechSynthesizePath
	defaultSpeechModel          = "speech-2.6-hd"
	defaultSpeechOutputFormat   = "hex"
)

type SpeechService struct {
	transport      *transport.Client
	endpoint       string
	streamEndpoint string
}

type SpeechRequest struct {
	Model   string   `json:"model,omitempty"`
	Text    string   `json:"text"`
	VoiceID string   `json:"voice_id,omitempty"`
	Speed   *float64 `json:"speed,omitempty"`
	Vol     *float64 `json:"vol,omitempty"`
	Pitch   *int     `json:"pitch,omitempty"`
}

type SpeechResponse struct {
	ResponseMeta ResponseMeta
	Audio        []byte
	RawHexAudio  string
}

type speechSynthesizeRawResponse struct {
	Data struct {
		AudioHex string `json:"audio_hex,omitempty"`
		Audio    string `json:"audio,omitempty"`
		Hex      string `json:"hex,omitempty"`
	} `json:"data"`
	AudioHex string `json:"audio_hex,omitempty"`
	Audio    string `json:"audio,omitempty"`
	Hex      string `json:"hex,omitempty"`
}

type speechSynthesizeWireRequest struct {
	Model        string              `json:"model"`
	Text         string              `json:"text"`
	OutputFormat string              `json:"output_format,omitempty"`
	VoiceSetting *speechVoiceSetting `json:"voice_setting,omitempty"`
}

type speechVoiceSetting struct {
	VoiceID string   `json:"voice_id,omitempty"`
	Speed   *float64 `json:"speed,omitempty"`
	Vol     *float64 `json:"vol,omitempty"`
	Pitch   *int     `json:"pitch,omitempty"`
}

// Synthesize performs sync TTS and returns decoded audio bytes.
func (s *SpeechService) Synthesize(ctx context.Context, request SpeechRequest) (*SpeechResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("speech service is not initialized")
	}

	request.Text = strings.TrimSpace(request.Text)
	request.Model = strings.TrimSpace(request.Model)
	request.VoiceID = strings.TrimSpace(request.VoiceID)

	if request.Text == "" {
		return nil, errors.New("speech request text is empty")
	}

	if request.Model == "" {
		request.Model = defaultSpeechModel
	}

	wireReq := speechSynthesizeWireRequest{
		Model:        request.Model,
		Text:         request.Text,
		OutputFormat: defaultSpeechOutputFormat,
	}

	if request.VoiceID != "" || request.Speed != nil || request.Vol != nil || request.Pitch != nil {
		wireReq.VoiceSetting = &speechVoiceSetting{
			VoiceID: request.VoiceID,
			Speed:   request.Speed,
			Vol:     request.Vol,
			Pitch:   request.Pitch,
		}
	}

	var raw speechSynthesizeRawResponse
	meta, err := s.transport.DoJSONWithMeta(ctx, transport.JSONRequest{
		Method: "POST",
		Path:   s.endpoint,
		Body:   wireReq,
	}, &raw)
	if err != nil {
		return nil, err
	}

	hexAudio := firstNonEmpty(
		raw.Data.AudioHex,
		raw.Data.Audio,
		raw.Data.Hex,
		raw.AudioHex,
		raw.Audio,
		raw.Hex,
	)
	if hexAudio == "" {
		return nil, errors.New("speech synthesize response missing hex audio payload")
	}

	audio, err := codec.DecodeHexAudio(hexAudio)
	if err != nil {
		return nil, fmt.Errorf("decode synthesized audio: %w", err)
	}

	return &SpeechResponse{
		ResponseMeta: responseMetaFromTransport(meta),
		Audio:        audio,
		RawHexAudio:  hexAudio,
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
