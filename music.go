package minimax

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/GizClaw/minimax-go/internal/transport"
)

const (
	defaultMusicGenerationPath      = "/v1/music_generation"
	defaultMusicCoverPreprocessPath = "/v1/music_cover_preprocess"
	defaultLyricsGenerationPath     = "/v1/lyrics_generation"
)

type MusicModel string

const (
	MusicModelV26       MusicModel = "music-2.6"
	MusicModelCover     MusicModel = "music-cover"
	MusicModelV26Free   MusicModel = "music-2.6-free"
	MusicModelCoverFree MusicModel = "music-cover-free"
)

type MusicOutputFormat string

const (
	MusicOutputFormatHex MusicOutputFormat = "hex"
	MusicOutputFormatURL MusicOutputFormat = "url"
)

type MusicAudioFormat string

const (
	MusicAudioFormatMP3 MusicAudioFormat = "mp3"
	MusicAudioFormatWAV MusicAudioFormat = "wav"
)

type LyricsMode string

const (
	LyricsModeWriteFullSong LyricsMode = "write_full_song"
	LyricsModeEdit          LyricsMode = "edit"
)

// MusicService provides MiniMax music generation APIs.
type MusicService struct {
	transport          *transport.Client
	generateEndpoint   string
	preprocessEndpoint string
	lyricsEndpoint     string
}

// MusicGenerateRequest contains parameters for MiniMax music and cover generation.
type MusicGenerateRequest struct {
	Model           string             `json:"model"`
	Prompt          string             `json:"prompt,omitempty"`
	Lyrics          string             `json:"lyrics,omitempty"`
	Stream          *bool              `json:"stream,omitempty"`
	OutputFormat    string             `json:"output_format,omitempty"`
	AudioSetting    *MusicAudioSetting `json:"audio_setting,omitempty"`
	AIGCWatermark   *bool              `json:"aigc_watermark,omitempty"`
	LyricsOptimizer *bool              `json:"lyrics_optimizer,omitempty"`
	IsInstrumental  *bool              `json:"is_instrumental,omitempty"`
	AudioURL        string             `json:"audio_url,omitempty"`
	AudioBase64     string             `json:"audio_base64,omitempty"`
	CoverFeatureID  string             `json:"cover_feature_id,omitempty"`
}

// MusicAudioSetting configures generated audio output.
type MusicAudioSetting struct {
	SampleRate *int   `json:"sample_rate,omitempty"`
	Bitrate    *int   `json:"bitrate,omitempty"`
	Format     string `json:"format,omitempty"`
}

// MusicGenerateResponse is a normalized non-streaming music generation response.
type MusicGenerateResponse struct {
	ResponseMeta ResponseMeta               `json:"response_meta,omitzero"`
	Audio        string                     `json:"audio,omitempty"`
	Status       *int                       `json:"status,omitempty"`
	ExtraInfo    MusicExtraInfo             `json:"extra_info"`
	TraceID      string                     `json:"trace_id,omitempty"`
	AnalysisInfo json.RawMessage            `json:"analysis_info,omitempty"`
	Raw          map[string]json.RawMessage `json:"-"`
}

// MusicExtraInfo describes generated audio metadata.
type MusicExtraInfo struct {
	MusicDuration   *int `json:"music_duration,omitempty"`
	MusicSampleRate *int `json:"music_sample_rate,omitempty"`
	MusicChannel    *int `json:"music_channel,omitempty"`
	Bitrate         *int `json:"bitrate,omitempty"`
	MusicSize       *int `json:"music_size,omitempty"`
}

// MusicCoverPreprocessRequest contains parameters for MiniMax cover preprocessing.
type MusicCoverPreprocessRequest struct {
	Model       string `json:"model"`
	AudioURL    string `json:"audio_url,omitempty"`
	AudioBase64 string `json:"audio_base64,omitempty"`
}

// MusicCoverPreprocessResponse contains the extracted cover feature and lyrics.
type MusicCoverPreprocessResponse struct {
	ResponseMeta    ResponseMeta               `json:"response_meta,omitzero"`
	CoverFeatureID  string                     `json:"cover_feature_id,omitempty"`
	FormattedLyrics string                     `json:"formatted_lyrics,omitempty"`
	StructureResult string                     `json:"structure_result,omitempty"`
	AudioDuration   *float64                   `json:"audio_duration,omitempty"`
	TraceID         string                     `json:"trace_id,omitempty"`
	Raw             map[string]json.RawMessage `json:"-"`
}

// LyricsGenerateRequest contains parameters for MiniMax lyrics generation.
type LyricsGenerateRequest struct {
	Mode   string `json:"mode"`
	Prompt string `json:"prompt,omitempty"`
	Lyrics string `json:"lyrics,omitempty"`
	Title  string `json:"title,omitempty"`
}

// LyricsGenerateResponse contains generated or edited lyrics.
type LyricsGenerateResponse struct {
	ResponseMeta ResponseMeta               `json:"response_meta,omitzero"`
	SongTitle    string                     `json:"song_title,omitempty"`
	StyleTags    string                     `json:"style_tags,omitempty"`
	Lyrics       string                     `json:"lyrics,omitempty"`
	Raw          map[string]json.RawMessage `json:"-"`
}

type musicGenerateRawResponse struct {
	Data         *musicGenerateRawData      `json:"data,omitempty"`
	Audio        string                     `json:"audio,omitempty"`
	Status       *int                       `json:"status,omitempty"`
	TraceID      string                     `json:"trace_id,omitempty"`
	ExtraInfo    MusicExtraInfo             `json:"extra_info"`
	AnalysisInfo json.RawMessage            `json:"analysis_info,omitempty"`
	Raw          map[string]json.RawMessage `json:"-"`
}

type musicGenerateRawData struct {
	Audio  string `json:"audio,omitempty"`
	Status *int   `json:"status,omitempty"`
}

type musicCoverPreprocessRawResponse struct {
	CoverFeatureID  string                     `json:"cover_feature_id,omitempty"`
	FormattedLyrics string                     `json:"formatted_lyrics,omitempty"`
	StructureResult string                     `json:"structure_result,omitempty"`
	AudioDuration   *float64                   `json:"audio_duration,omitempty"`
	TraceID         string                     `json:"trace_id,omitempty"`
	Raw             map[string]json.RawMessage `json:"-"`
}

type lyricsGenerateRawResponse struct {
	SongTitle string                     `json:"song_title,omitempty"`
	StyleTags string                     `json:"style_tags,omitempty"`
	Lyrics    string                     `json:"lyrics,omitempty"`
	Raw       map[string]json.RawMessage `json:"-"`
}

// Generate generates music or cover music using MiniMax music_generation.
func (s *MusicService) Generate(ctx context.Context, request MusicGenerateRequest) (*MusicGenerateResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("music service is not initialized")
	}

	normalizeMusicGenerateRequest(&request)
	if err := validateMusicGenerateRequest(request); err != nil {
		return nil, err
	}

	var raw musicGenerateRawResponse
	meta, err := s.transport.DoJSONWithMeta(ctx, transport.JSONRequest{
		Method: http.MethodPost,
		Path:   s.resolveGeneratePath(),
		Body:   request,
	}, &raw)
	if err != nil {
		return nil, err
	}

	response := mapMusicGenerateResponse(raw)
	response.ResponseMeta = responseMetaFromTransport(meta)
	return response, nil
}

// PreprocessCover preprocesses reference audio for two-step cover generation.
func (s *MusicService) PreprocessCover(ctx context.Context, request MusicCoverPreprocessRequest) (*MusicCoverPreprocessResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("music service is not initialized")
	}

	normalizeMusicCoverPreprocessRequest(&request)
	if err := validateMusicCoverPreprocessRequest(request); err != nil {
		return nil, err
	}

	var raw musicCoverPreprocessRawResponse
	meta, err := s.transport.DoJSONWithMeta(ctx, transport.JSONRequest{
		Method: http.MethodPost,
		Path:   s.resolvePreprocessPath(),
		Body:   request,
	}, &raw)
	if err != nil {
		return nil, err
	}

	return &MusicCoverPreprocessResponse{
		ResponseMeta:    responseMetaFromTransport(meta),
		CoverFeatureID:  strings.TrimSpace(raw.CoverFeatureID),
		FormattedLyrics: raw.FormattedLyrics,
		StructureResult: raw.StructureResult,
		AudioDuration:   raw.AudioDuration,
		TraceID:         strings.TrimSpace(raw.TraceID),
		Raw:             cloneRawMessages(raw.Raw),
	}, nil
}

// GenerateLyrics generates or edits lyrics for music generation workflows.
func (s *MusicService) GenerateLyrics(ctx context.Context, request LyricsGenerateRequest) (*LyricsGenerateResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("music service is not initialized")
	}

	normalizeLyricsGenerateRequest(&request)
	if err := validateLyricsGenerateRequest(request); err != nil {
		return nil, err
	}

	var raw lyricsGenerateRawResponse
	meta, err := s.transport.DoJSONWithMeta(ctx, transport.JSONRequest{
		Method: http.MethodPost,
		Path:   s.resolveLyricsPath(),
		Body:   request,
	}, &raw)
	if err != nil {
		return nil, err
	}

	return &LyricsGenerateResponse{
		ResponseMeta: responseMetaFromTransport(meta),
		SongTitle:    strings.TrimSpace(raw.SongTitle),
		StyleTags:    strings.TrimSpace(raw.StyleTags),
		Lyrics:       raw.Lyrics,
		Raw:          cloneRawMessages(raw.Raw),
	}, nil
}

func normalizeMusicGenerateRequest(request *MusicGenerateRequest) {
	request.Model = strings.TrimSpace(request.Model)
	request.Prompt = strings.TrimSpace(request.Prompt)
	request.OutputFormat = strings.TrimSpace(request.OutputFormat)
	request.AudioURL = strings.TrimSpace(request.AudioURL)
	request.AudioBase64 = strings.TrimSpace(request.AudioBase64)
	request.CoverFeatureID = strings.TrimSpace(request.CoverFeatureID)
	if request.AudioSetting != nil {
		request.AudioSetting.Format = strings.TrimSpace(request.AudioSetting.Format)
	}
}

func normalizeMusicCoverPreprocessRequest(request *MusicCoverPreprocessRequest) {
	request.Model = strings.TrimSpace(request.Model)
	request.AudioURL = strings.TrimSpace(request.AudioURL)
	request.AudioBase64 = strings.TrimSpace(request.AudioBase64)
}

func normalizeLyricsGenerateRequest(request *LyricsGenerateRequest) {
	request.Mode = strings.TrimSpace(request.Mode)
	request.Prompt = strings.TrimSpace(request.Prompt)
	request.Title = strings.TrimSpace(request.Title)
}

func validateMusicGenerateRequest(request MusicGenerateRequest) error {
	if request.Model == "" {
		return errors.New("music generate request model is empty")
	}
	if !isSupportedMusicModel(request.Model) {
		return fmt.Errorf("music generate request model is not supported: %s", request.Model)
	}
	if request.Stream != nil && *request.Stream {
		return errors.New("music generate request stream=true is not supported")
	}
	if request.OutputFormat != "" && request.OutputFormat != string(MusicOutputFormatHex) && request.OutputFormat != string(MusicOutputFormatURL) {
		return fmt.Errorf("music generate request output_format must be hex or url: %s", request.OutputFormat)
	}
	if request.AudioSetting != nil && request.AudioSetting.Format != "" && request.AudioSetting.Format != string(MusicAudioFormatMP3) && request.AudioSetting.Format != string(MusicAudioFormatWAV) {
		return fmt.Errorf("music generate request audio_setting.format must be mp3 or wav: %s", request.AudioSetting.Format)
	}

	if isMusicCoverModel(request.Model) {
		return validateMusicCoverGenerateRequest(request)
	}
	return validateMusicSongGenerateRequest(request)
}

func validateMusicSongGenerateRequest(request MusicGenerateRequest) error {
	if request.AudioURL != "" || request.AudioBase64 != "" || request.CoverFeatureID != "" {
		return errors.New("music generate request audio_url, audio_base64, and cover_feature_id require a music-cover model")
	}
	if request.IsInstrumental != nil && *request.IsInstrumental {
		if request.Prompt == "" {
			return errors.New("music generate request prompt is empty for instrumental generation")
		}
		return nil
	}
	if request.LyricsOptimizer != nil && *request.LyricsOptimizer && request.Lyrics == "" {
		if request.Prompt == "" {
			return errors.New("music generate request prompt is empty when lyrics_optimizer is true")
		}
		return nil
	}
	if strings.TrimSpace(request.Lyrics) == "" {
		return errors.New("music generate request lyrics is empty")
	}
	return nil
}

func validateMusicCoverGenerateRequest(request MusicGenerateRequest) error {
	if request.LyricsOptimizer != nil && *request.LyricsOptimizer {
		return errors.New("music cover request lyrics_optimizer is only supported by music-2.6 models")
	}
	if request.IsInstrumental != nil && *request.IsInstrumental {
		return errors.New("music cover request is_instrumental is only supported by music-2.6 models")
	}

	sources := 0
	if request.AudioURL != "" {
		sources++
	}
	if request.AudioBase64 != "" {
		sources++
	}
	if request.CoverFeatureID != "" {
		sources++
	}
	if sources != 1 {
		return errors.New("music cover request requires exactly one of audio_url, audio_base64, or cover_feature_id")
	}
	if request.Prompt == "" {
		return errors.New("music cover request prompt is empty")
	}
	if request.CoverFeatureID != "" && strings.TrimSpace(request.Lyrics) == "" {
		return errors.New("music cover request lyrics is empty when cover_feature_id is set")
	}
	return nil
}

func validateMusicCoverPreprocessRequest(request MusicCoverPreprocessRequest) error {
	if request.Model == "" {
		return errors.New("music cover preprocess request model is empty")
	}
	if request.Model != string(MusicModelCover) {
		return fmt.Errorf("music cover preprocess request model must be %q: %s", MusicModelCover, request.Model)
	}
	hasURL := request.AudioURL != ""
	hasBase64 := request.AudioBase64 != ""
	if hasURL == hasBase64 {
		return errors.New("music cover preprocess request requires exactly one of audio_url or audio_base64")
	}
	return nil
}

func validateLyricsGenerateRequest(request LyricsGenerateRequest) error {
	switch request.Mode {
	case string(LyricsModeWriteFullSong):
		return nil
	case string(LyricsModeEdit):
		if strings.TrimSpace(request.Lyrics) == "" {
			return errors.New("lyrics generate request lyrics is empty for edit mode")
		}
		return nil
	case "":
		return errors.New("lyrics generate request mode is empty")
	default:
		return fmt.Errorf("lyrics generate request mode must be write_full_song or edit: %s", request.Mode)
	}
}

func isSupportedMusicModel(model string) bool {
	switch model {
	case string(MusicModelV26), string(MusicModelCover), string(MusicModelV26Free), string(MusicModelCoverFree):
		return true
	default:
		return false
	}
}

func isMusicCoverModel(model string) bool {
	return model == string(MusicModelCover) || model == string(MusicModelCoverFree)
}

func mapMusicGenerateResponse(raw musicGenerateRawResponse) *MusicGenerateResponse {
	response := &MusicGenerateResponse{
		Audio:        strings.TrimSpace(raw.Audio),
		Status:       raw.Status,
		TraceID:      strings.TrimSpace(raw.TraceID),
		ExtraInfo:    raw.ExtraInfo,
		AnalysisInfo: cloneRawMessage(raw.AnalysisInfo),
		Raw:          cloneRawMessages(raw.Raw),
	}
	if raw.Data != nil {
		response.Audio = firstNonEmptyValue(response.Audio, strings.TrimSpace(raw.Data.Audio))
		if response.Status == nil {
			response.Status = raw.Data.Status
		}
	}
	return response
}

func (s *MusicService) resolveGeneratePath() string {
	if s.generateEndpoint != "" {
		return s.generateEndpoint
	}
	return defaultMusicGenerationPath
}

func (s *MusicService) resolvePreprocessPath() string {
	if s.preprocessEndpoint != "" {
		return s.preprocessEndpoint
	}
	return defaultMusicCoverPreprocessPath
}

func (s *MusicService) resolveLyricsPath() string {
	if s.lyricsEndpoint != "" {
		return s.lyricsEndpoint
	}
	return defaultLyricsGenerationPath
}

func (r *musicGenerateRawResponse) UnmarshalJSON(data []byte) error {
	type alias musicGenerateRawResponse
	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	raw, err := rawMessagesWithout(data, "data", "audio", "status", "trace_id", "extra_info", "analysis_info", "base_resp", "status_code", "status_msg")
	if err != nil {
		return err
	}

	*r = musicGenerateRawResponse(parsed)
	r.Raw = raw
	return nil
}

func (r *musicCoverPreprocessRawResponse) UnmarshalJSON(data []byte) error {
	type alias musicCoverPreprocessRawResponse
	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	raw, err := rawMessagesWithout(data, "cover_feature_id", "formatted_lyrics", "structure_result", "audio_duration", "trace_id", "base_resp", "status_code", "status_msg")
	if err != nil {
		return err
	}

	*r = musicCoverPreprocessRawResponse(parsed)
	r.Raw = raw
	return nil
}

func (r *lyricsGenerateRawResponse) UnmarshalJSON(data []byte) error {
	type alias lyricsGenerateRawResponse
	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	raw, err := rawMessagesWithout(data, "song_title", "style_tags", "lyrics", "base_resp", "status_code", "status_msg")
	if err != nil {
		return err
	}

	*r = lyricsGenerateRawResponse(parsed)
	r.Raw = raw
	return nil
}

func rawMessagesWithout(data []byte, keys ...string) (map[string]json.RawMessage, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	for _, key := range keys {
		delete(raw, key)
	}
	if len(raw) == 0 {
		return nil, nil
	}
	return raw, nil
}

func cloneRawMessage(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), value...)
}
