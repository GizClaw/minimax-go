package minimax

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/GizClaw/minimax-go/internal/transport"
)

const (
	defaultVoiceListPath   = "/v1/get_voice"
	defaultVoiceDesignPath = "/v1/voice_design"
	defaultVoiceClonePath  = "/v1/voice_clone"
	defaultVoiceType       = "all"
)

type VoiceService struct {
	transport      *transport.Client
	endpoint       string
	designEndpoint string
	cloneEndpoint  string
}

type ListVoicesRequest struct {
	VoiceType string `json:"voice_type,omitempty"`
	PageSize  *int   `json:"page_size,omitempty"`
	PageToken string `json:"page_token,omitempty"`
}

type DesignVoiceRequest struct {
	Prompt      string `json:"prompt"`
	PreviewText string `json:"preview_text"`
	VoiceID     string `json:"voice_id,omitempty"`
}

type CloneVoiceRequest struct {
	VoiceID  string `json:"voice_id"`
	AudioURL string `json:"audio_url,omitempty"`
	FileID   string `json:"file_id,omitempty"`
}

type Voice struct {
	VoiceID     string                     `json:"voice_id,omitempty"`
	VoiceName   string                     `json:"voice_name,omitempty"`
	Description []string                   `json:"description,omitempty"`
	CreatedTime string                     `json:"created_time,omitempty"`
	VoiceType   string                     `json:"voice_type,omitempty"`
	Raw         map[string]json.RawMessage `json:"-"`
}

type ListVoicesResponse struct {
	Voices        []Voice                    `json:"voices"`
	NextPageToken string                     `json:"next_page_token,omitempty"`
	HasMore       bool                       `json:"has_more,omitempty"`
	Raw           map[string]json.RawMessage `json:"-"`
}

type DesignVoiceResponse struct {
	VoiceID    string                     `json:"voice_id,omitempty"`
	TrialAudio string                     `json:"trial_audio,omitempty"`
	Raw        map[string]json.RawMessage `json:"-"`
}

type CloneVoiceResponse struct {
	VoiceID   string                     `json:"voice_id,omitempty"`
	DemoAudio string                     `json:"demo_audio,omitempty"`
	Raw       map[string]json.RawMessage `json:"-"`
}

type listVoicesWireRequest struct {
	VoiceType string `json:"voice_type"`
	PageSize  *int   `json:"page_size,omitempty"`
	PageToken string `json:"page_token,omitempty"`
}

type designVoiceWireRequest struct {
	Prompt      string `json:"prompt"`
	PreviewText string `json:"preview_text"`
	VoiceID     string `json:"voice_id,omitempty"`
}

type cloneVoiceWireRequest struct {
	VoiceID  string      `json:"voice_id"`
	AudioURL string      `json:"audio_url,omitempty"`
	FileID   cloneFileID `json:"file_id,omitempty"`
}

type cloneFileID string

type listVoicesRawResponse struct {
	Voices          []Voice                    `json:"voices,omitempty"`
	SystemVoice     []Voice                    `json:"system_voice,omitempty"`
	VoiceCloning    []Voice                    `json:"voice_cloning,omitempty"`
	VoiceGeneration []Voice                    `json:"voice_generation,omitempty"`
	NextPageToken   string                     `json:"next_page_token,omitempty"`
	HasMore         bool                       `json:"has_more,omitempty"`
	Raw             map[string]json.RawMessage `json:"-"`
}

type designVoiceRawResponse struct {
	VoiceID      string                     `json:"voice_id,omitempty"`
	CustomID     string                     `json:"custom_voice_id,omitempty"`
	TrialAudio   string                     `json:"trial_audio,omitempty"`
	DemoAudio    string                     `json:"demo_audio,omitempty"`
	PreviewAudio string                     `json:"preview_audio,omitempty"`
	Raw          map[string]json.RawMessage `json:"-"`
}

type cloneVoiceRawResponse struct {
	VoiceID      string                     `json:"voice_id,omitempty"`
	CustomID     string                     `json:"custom_voice_id,omitempty"`
	DemoAudio    string                     `json:"demo_audio,omitempty"`
	TrialAudio   string                     `json:"trial_audio,omitempty"`
	PreviewAudio string                     `json:"preview_audio,omitempty"`
	Raw          map[string]json.RawMessage `json:"-"`
}

// ListVoices queries available voices with filter and pagination parameters.
func (s *VoiceService) ListVoices(ctx context.Context, request *ListVoicesRequest) (*ListVoicesResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("voice service is not initialized")
	}

	payload, query, err := buildListVoicesPayload(request)
	if err != nil {
		return nil, err
	}

	var raw listVoicesRawResponse
	if err := s.transport.DoJSON(ctx, transport.JSONRequest{
		Method: http.MethodPost,
		Path:   s.resolveListPath(),
		Query:  query,
		Body:   payload,
	}, &raw); err != nil {
		return nil, err
	}

	return &ListVoicesResponse{
		Voices:        collectVoices(raw),
		NextPageToken: strings.TrimSpace(raw.NextPageToken),
		HasMore:       raw.HasMore,
		Raw:           cloneRawMessages(raw.Raw),
	}, nil
}

// DesignVoice creates a new custom voice based on prompt and preview text.
func (s *VoiceService) DesignVoice(ctx context.Context, request *DesignVoiceRequest) (*DesignVoiceResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("voice service is not initialized")
	}

	payload, err := buildDesignVoicePayload(request)
	if err != nil {
		return nil, err
	}

	var raw designVoiceRawResponse
	if err := s.transport.DoJSON(ctx, transport.JSONRequest{
		Method: http.MethodPost,
		Path:   s.resolveDesignPath(),
		Body:   payload,
	}, &raw); err != nil {
		return nil, err
	}

	return &DesignVoiceResponse{
		VoiceID:    firstNonEmptyValue(raw.VoiceID, raw.CustomID, payload.VoiceID),
		TrialAudio: firstNonEmptyValue(raw.TrialAudio, raw.DemoAudio, raw.PreviewAudio),
		Raw:        cloneRawMessages(raw.Raw),
	}, nil
}

// CloneVoice clones a voice from either an audio URL or a previously uploaded file_id.
func (s *VoiceService) CloneVoice(ctx context.Context, request *CloneVoiceRequest) (*CloneVoiceResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("voice service is not initialized")
	}

	payload, err := buildCloneVoicePayload(request)
	if err != nil {
		return nil, err
	}

	var raw cloneVoiceRawResponse
	if err := s.transport.DoJSON(ctx, transport.JSONRequest{
		Method: http.MethodPost,
		Path:   s.resolveClonePath(),
		Body:   payload,
	}, &raw); err != nil {
		return nil, err
	}

	return &CloneVoiceResponse{
		VoiceID:   firstNonEmptyValue(raw.VoiceID, raw.CustomID, payload.VoiceID),
		DemoAudio: firstNonEmptyValue(raw.DemoAudio, raw.TrialAudio, raw.PreviewAudio),
		Raw:       cloneRawMessages(raw.Raw),
	}, nil
}

func buildDesignVoicePayload(request *DesignVoiceRequest) (designVoiceWireRequest, error) {
	if request == nil {
		return designVoiceWireRequest{}, errors.New("design voice request is nil")
	}

	payload := designVoiceWireRequest{
		Prompt:      strings.TrimSpace(request.Prompt),
		PreviewText: strings.TrimSpace(request.PreviewText),
		VoiceID:     strings.TrimSpace(request.VoiceID),
	}

	if payload.Prompt == "" {
		return designVoiceWireRequest{}, errors.New("design voice request prompt is empty")
	}

	if payload.PreviewText == "" {
		return designVoiceWireRequest{}, errors.New("design voice request preview_text is empty")
	}

	return payload, nil
}

func buildCloneVoicePayload(request *CloneVoiceRequest) (cloneVoiceWireRequest, error) {
	if request == nil {
		return cloneVoiceWireRequest{}, errors.New("clone voice request is nil")
	}

	payload := cloneVoiceWireRequest{
		VoiceID:  strings.TrimSpace(request.VoiceID),
		AudioURL: strings.TrimSpace(request.AudioURL),
		FileID:   cloneFileID(strings.TrimSpace(request.FileID)),
	}

	if payload.VoiceID == "" {
		return cloneVoiceWireRequest{}, errors.New("clone voice request voice_id is empty")
	}

	if payload.AudioURL == "" && payload.FileID == "" {
		return cloneVoiceWireRequest{}, errors.New("clone voice request requires at least one input source: audio_url or file_id")
	}

	return payload, nil
}

func (id cloneFileID) MarshalJSON() ([]byte, error) {
	trimmed := strings.TrimSpace(string(id))
	if trimmed == "" {
		return json.Marshal("")
	}

	if shouldEncodeCloneFileIDAsNumber(trimmed) {
		return []byte(trimmed), nil
	}

	return json.Marshal(trimmed)
}

func shouldEncodeCloneFileIDAsNumber(value string) bool {
	if !isDigitsOnly(value) {
		return false
	}

	// JSON number tokens cannot have leading zeros (except the literal "0").
	// Keep opaque identifiers with leading zeros as strings to avoid invalid JSON
	// and preserve source token semantics.
	if len(value) > 1 && value[0] == '0' {
		return false
	}

	return true
}

func isDigitsOnly(value string) bool {
	if value == "" {
		return false
	}

	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

func buildListVoicesPayload(request *ListVoicesRequest) (listVoicesWireRequest, url.Values, error) {
	payload := listVoicesWireRequest{VoiceType: defaultVoiceType}
	if request != nil {
		payload.VoiceType = strings.TrimSpace(request.VoiceType)
		payload.PageSize = request.PageSize
		payload.PageToken = strings.TrimSpace(request.PageToken)
	}

	if payload.VoiceType == "" {
		payload.VoiceType = defaultVoiceType
	}

	if payload.PageSize != nil && *payload.PageSize < 0 {
		return listVoicesWireRequest{}, nil, errors.New("list voices request page_size must be non-negative")
	}

	query := make(url.Values)
	query.Set("voice_type", payload.VoiceType)
	if payload.PageSize != nil {
		query.Set("page_size", strconv.Itoa(*payload.PageSize))
	}
	if payload.PageToken != "" {
		query.Set("page_token", payload.PageToken)
	}

	return payload, query, nil
}

func (s *VoiceService) resolveListPath() string {
	listPath := strings.TrimSpace(s.endpoint)
	if listPath != "" {
		return listPath
	}

	return defaultVoiceListPath
}

func (s *VoiceService) resolveDesignPath() string {
	designPath := strings.TrimSpace(s.designEndpoint)
	if designPath != "" {
		return designPath
	}

	return defaultVoiceDesignPath
}

func (s *VoiceService) resolveClonePath() string {
	clonePath := strings.TrimSpace(s.cloneEndpoint)
	if clonePath != "" {
		return clonePath
	}

	return defaultVoiceClonePath
}

func collectVoices(raw listVoicesRawResponse) []Voice {
	if raw.Voices != nil {
		return cloneVoices(raw.Voices, "")
	}

	voices := make([]Voice, 0, len(raw.SystemVoice)+len(raw.VoiceCloning)+len(raw.VoiceGeneration))
	voices = appendVoices(voices, raw.SystemVoice, "system")
	voices = appendVoices(voices, raw.VoiceCloning, "voice_cloning")
	voices = appendVoices(voices, raw.VoiceGeneration, "voice_generation")

	if voices == nil {
		return make([]Voice, 0)
	}

	return voices
}

func appendVoices(dst, src []Voice, voiceType string) []Voice {
	for _, item := range src {
		copied := item
		if copied.VoiceType == "" {
			copied.VoiceType = voiceType
		}
		copied.Raw = cloneRawMessages(copied.Raw)
		dst = append(dst, copied)
	}
	return dst
}

func cloneVoices(voices []Voice, defaultType string) []Voice {
	if voices == nil {
		return make([]Voice, 0)
	}

	cloned := make([]Voice, 0, len(voices))
	for _, item := range voices {
		copied := item
		if copied.VoiceType == "" && defaultType != "" {
			copied.VoiceType = defaultType
		}
		copied.Raw = cloneRawMessages(copied.Raw)
		cloned = append(cloned, copied)
	}

	return cloned
}

func cloneRawMessages(raw map[string]json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}

	cloned := make(map[string]json.RawMessage, len(raw))
	for key, value := range raw {
		clonedValue := make(json.RawMessage, len(value))
		copy(clonedValue, value)
		cloned[key] = clonedValue
	}

	return cloned
}

func (v *Voice) UnmarshalJSON(data []byte) error {
	type alias Voice

	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	delete(raw, "voice_id")
	delete(raw, "voice_name")
	delete(raw, "description")
	delete(raw, "created_time")
	delete(raw, "voice_type")

	*v = Voice(parsed)
	if len(raw) > 0 {
		v.Raw = raw
	} else {
		v.Raw = nil
	}

	return nil
}

func (r *listVoicesRawResponse) UnmarshalJSON(data []byte) error {
	type alias listVoicesRawResponse

	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	delete(raw, "voices")
	delete(raw, "system_voice")
	delete(raw, "voice_cloning")
	delete(raw, "voice_generation")
	delete(raw, "next_page_token")
	delete(raw, "has_more")
	delete(raw, "base_resp")
	delete(raw, "status_code")
	delete(raw, "status_msg")

	*r = listVoicesRawResponse(parsed)
	if len(raw) > 0 {
		r.Raw = raw
	} else {
		r.Raw = nil
	}

	return nil
}

func (r *designVoiceRawResponse) UnmarshalJSON(data []byte) error {
	type alias designVoiceRawResponse

	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	delete(raw, "voice_id")
	delete(raw, "custom_voice_id")
	delete(raw, "trial_audio")
	delete(raw, "demo_audio")
	delete(raw, "preview_audio")
	delete(raw, "base_resp")
	delete(raw, "status_code")
	delete(raw, "status_msg")

	*r = designVoiceRawResponse(parsed)
	if len(raw) > 0 {
		r.Raw = raw
	} else {
		r.Raw = nil
	}

	return nil
}

func (r *cloneVoiceRawResponse) UnmarshalJSON(data []byte) error {
	type alias cloneVoiceRawResponse

	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	delete(raw, "voice_id")
	delete(raw, "custom_voice_id")
	delete(raw, "demo_audio")
	delete(raw, "trial_audio")
	delete(raw, "preview_audio")
	delete(raw, "base_resp")
	delete(raw, "status_code")
	delete(raw, "status_msg")

	*r = cloneVoiceRawResponse(parsed)
	if len(raw) > 0 {
		r.Raw = raw
	} else {
		r.Raw = nil
	}

	return nil
}
