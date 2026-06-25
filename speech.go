package minimax

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
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
	websocketPath  string
}

type SpeechRequest struct {
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
	AIGCWatermark        *bool                    `json:"aigc_watermark,omitempty"`
}

type SpeechResponse struct {
	ResponseMeta ResponseMeta               `json:"response_meta,omitzero"`
	Audio        []byte                     `json:"audio,omitempty"`
	RawHexAudio  string                     `json:"raw_hex_audio,omitempty"`
	AudioURL     string                     `json:"audio_url,omitempty"`
	TraceID      string                     `json:"trace_id,omitempty"`
	Raw          map[string]json.RawMessage `json:"-"`
}

type SpeechAudioSetting struct {
	SampleRate *int   `json:"sample_rate,omitempty"`
	Bitrate    *int   `json:"bitrate,omitempty"`
	Format     string `json:"format,omitempty"`
	Channel    *int   `json:"channel,omitempty"`
}

type SpeechPronunciationDict struct {
	Tone []string `json:"tone,omitempty"`
}

type SpeechTimberWeight struct {
	VoiceID string `json:"voice_id,omitempty"`
	Weight  int    `json:"weight,omitempty"`
}

type SpeechVoiceModify struct {
	Pitch        *int   `json:"pitch,omitempty"`
	Intensity    *int   `json:"intensity,omitempty"`
	Timbre       *int   `json:"timbre,omitempty"`
	SoundEffects string `json:"sound_effects,omitempty"`
}

type speechSynthesizeRawResponse struct {
	Data struct {
		AudioHex string `json:"audio_hex,omitempty"`
		Audio    string `json:"audio,omitempty"`
		Hex      string `json:"hex,omitempty"`
		URL      string `json:"url,omitempty"`
		AudioURL string `json:"audio_url,omitempty"`
		FileURL  string `json:"file_url,omitempty"`
	} `json:"data"`
	AudioHex string                     `json:"audio_hex,omitempty"`
	Audio    string                     `json:"audio,omitempty"`
	Hex      string                     `json:"hex,omitempty"`
	URL      string                     `json:"url,omitempty"`
	AudioURL string                     `json:"audio_url,omitempty"`
	FileURL  string                     `json:"file_url,omitempty"`
	TraceID  string                     `json:"trace_id,omitempty"`
	Raw      map[string]json.RawMessage `json:"-"`
}

type speechSynthesizeWireRequest struct {
	Model             string                   `json:"model"`
	Text              string                   `json:"text"`
	Stream            bool                     `json:"stream"`
	OutputFormat      string                   `json:"output_format,omitempty"`
	VoiceSetting      *speechVoiceSetting      `json:"voice_setting,omitempty"`
	AudioSetting      *speechAudioSettingWire  `json:"audio_setting,omitempty"`
	PronunciationDict *SpeechPronunciationDict `json:"pronunciation_dict,omitempty"`
	TimberWeights     []SpeechTimberWeight     `json:"timbre_weights,omitempty"`
	LanguageBoost     string                   `json:"language_boost,omitempty"`
	VoiceModify       *SpeechVoiceModify       `json:"voice_modify,omitempty"`
	SubtitleEnable    *bool                    `json:"subtitle_enable,omitempty"`
	SubtitleType      string                   `json:"subtitle_type,omitempty"`
	AIGCWatermark     *bool                    `json:"aigc_watermark,omitempty"`
}

type speechVoiceSetting struct {
	VoiceID              string   `json:"voice_id,omitempty"`
	Speed                *float64 `json:"speed,omitempty"`
	Vol                  *float64 `json:"vol,omitempty"`
	Pitch                *int     `json:"pitch,omitempty"`
	Emotion              string   `json:"emotion,omitempty"`
	EnglishNormalization *bool    `json:"english_normalization,omitempty"`
	LatexRead            *bool    `json:"latex_read,omitempty"`
}

type speechAudioSettingWire struct {
	SampleRate      *int   `json:"sample_rate,omitempty"`
	AudioSampleRate *int   `json:"audio_sample_rate,omitempty"`
	Bitrate         *int   `json:"bitrate,omitempty"`
	Format          string `json:"format,omitempty"`
	Channel         *int   `json:"channel,omitempty"`
}

// Synthesize performs sync TTS and returns decoded audio bytes.
func (s *SpeechService) Synthesize(ctx context.Context, request SpeechRequest) (*SpeechResponse, error) {
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
		return nil, errors.New("speech request text is empty")
	}

	if request.Model == "" {
		request.Model = defaultSpeechModel
	}
	if request.OutputFormat == "" {
		request.OutputFormat = defaultSpeechOutputFormat
	}
	if !isSupportedSpeechOutputFormat(request.OutputFormat) {
		return nil, fmt.Errorf("speech output format %q is not supported, only %q and %q are supported", request.OutputFormat, "hex", "url")
	}

	wireReq := speechSynthesizeWireRequest{
		Model:             request.Model,
		Text:              request.Text,
		Stream:            false,
		OutputFormat:      strings.ToLower(request.OutputFormat),
		AudioSetting:      speechAudioSettingForHTTP(request.AudioSetting),
		PronunciationDict: request.PronunciationDict,
		TimberWeights:     trimSpeechTimberWeights(request.TimberWeights),
		LanguageBoost:     request.LanguageBoost,
		VoiceModify:       request.VoiceModify,
		SubtitleEnable:    request.SubtitleEnable,
		SubtitleType:      request.SubtitleType,
		AIGCWatermark:     request.AIGCWatermark,
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

	var raw speechSynthesizeRawResponse
	meta, err := s.transport.DoJSONWithMeta(ctx, transport.JSONRequest{
		Method: "POST",
		Path:   s.endpoint,
		Body:   wireReq,
	}, &raw)
	if err != nil {
		return nil, err
	}

	audioURL := synthesizeAudioURL(raw)
	if audioURL != "" {
		return &SpeechResponse{
			ResponseMeta: responseMetaFromTransport(meta),
			AudioURL:     audioURL,
			TraceID:      strings.TrimSpace(raw.TraceID),
			Raw:          cloneRawMessages(raw.Raw),
		}, nil
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
		return nil, errors.New("speech synthesize response missing audio payload")
	}

	audio, err := codec.DecodeHexAudio(hexAudio)
	if err != nil {
		return nil, fmt.Errorf("decode synthesized audio: %w", err)
	}

	return &SpeechResponse{
		ResponseMeta: responseMetaFromTransport(meta),
		Audio:        audio,
		RawHexAudio:  hexAudio,
		TraceID:      strings.TrimSpace(raw.TraceID),
		Raw:          cloneRawMessages(raw.Raw),
	}, nil
}

func synthesizeAudioURL(raw speechSynthesizeRawResponse) string {
	audioURL := firstNonEmpty(
		raw.Data.AudioURL,
		raw.Data.URL,
		raw.Data.FileURL,
		raw.AudioURL,
		raw.URL,
		raw.FileURL,
	)
	if audioURL != "" {
		return audioURL
	}

	for _, candidate := range []string{
		raw.Data.Audio,
		raw.Data.AudioHex,
		raw.Data.Hex,
		raw.Audio,
		raw.AudioHex,
		raw.Hex,
	} {
		if isHTTPURL(candidate) {
			return strings.TrimSpace(candidate)
		}
	}

	return ""
}

func isHTTPURL(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}

	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func isSupportedSpeechOutputFormat(format string) bool {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "hex", "url":
		return true
	default:
		return false
	}
}

func speechAudioSettingForHTTP(setting *SpeechAudioSetting) *speechAudioSettingWire {
	return speechAudioSettingWireFromPublic(setting, false)
}

func speechAudioSettingForAsync(setting *SpeechAudioSetting) *speechAudioSettingWire {
	return speechAudioSettingWireFromPublic(setting, true)
}

func speechAudioSettingWireFromPublic(setting *SpeechAudioSetting, async bool) *speechAudioSettingWire {
	if setting == nil {
		return nil
	}

	wire := &speechAudioSettingWire{
		Bitrate: setting.Bitrate,
		Format:  strings.TrimSpace(setting.Format),
		Channel: setting.Channel,
	}
	if async {
		wire.AudioSampleRate = setting.SampleRate
	} else {
		wire.SampleRate = setting.SampleRate
	}
	return wire
}

func trimSpeechTimberWeights(weights []SpeechTimberWeight) []SpeechTimberWeight {
	if len(weights) == 0 {
		return nil
	}

	trimmed := make([]SpeechTimberWeight, 0, len(weights))
	for _, weight := range weights {
		weight.VoiceID = strings.TrimSpace(weight.VoiceID)
		if weight.VoiceID == "" && weight.Weight == 0 {
			continue
		}
		trimmed = append(trimmed, weight)
	}
	if len(trimmed) == 0 {
		return nil
	}
	return trimmed
}

func (r *speechSynthesizeRawResponse) UnmarshalJSON(data []byte) error {
	type alias speechSynthesizeRawResponse

	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	delete(raw, "data")
	delete(raw, "audio_hex")
	delete(raw, "audio")
	delete(raw, "hex")
	delete(raw, "url")
	delete(raw, "audio_url")
	delete(raw, "file_url")
	delete(raw, "trace_id")
	delete(raw, "base_resp")
	delete(raw, "status_code")
	delete(raw, "status_msg")

	*r = speechSynthesizeRawResponse(parsed)
	if len(raw) > 0 {
		r.Raw = raw
	} else {
		r.Raw = nil
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
