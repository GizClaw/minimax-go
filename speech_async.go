package minimax

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/GizClaw/minimax-go/internal/codec"
	"github.com/GizClaw/minimax-go/internal/transport"
)

const (
	defaultSpeechAsyncSubmitPath = "/v1/t2a_async_v2"
	defaultSpeechAsyncQueryPath  = "/v1/query/t2a_async_query_v2"
)

// SpeechTaskState is the normalized async speech task state.
type SpeechTaskState string

const (
	SpeechTaskStateQueued    SpeechTaskState = "queued"
	SpeechTaskStateRunning   SpeechTaskState = "running"
	SpeechTaskStateSucceeded SpeechTaskState = "succeeded"
	SpeechTaskStateFailed    SpeechTaskState = "failed"
)

// IsTerminal reports whether the task state is terminal.
func (s SpeechTaskState) IsTerminal() bool {
	return s == SpeechTaskStateSucceeded || s == SpeechTaskStateFailed
}

type SpeechAsyncService struct {
	transport      *transport.Client
	submitEndpoint string
	queryEndpoint  string
}

type SpeechAsyncSubmitRequest struct {
	Model      string   `json:"model,omitempty"`
	Text       string   `json:"text,omitempty"`
	TextFileID string   `json:"text_file_id,omitempty"`
	VoiceID    string   `json:"voice_id,omitempty"`
	Speed      *float64 `json:"speed,omitempty"`
	Vol        *float64 `json:"vol,omitempty"`
	Pitch      *int     `json:"pitch,omitempty"`
}

type SpeechAsyncSubmitResponse struct {
	TaskID          string                     `json:"task_id"`
	Status          SpeechTaskState            `json:"status,omitempty"`
	RawStatus       string                     `json:"raw_status,omitempty"`
	FileID          string                     `json:"file_id,omitempty"`
	TaskToken       string                     `json:"task_token,omitempty"`
	UsageCharacters *int64                     `json:"usage_characters,omitempty"`
	Raw             map[string]json.RawMessage `json:"-"`
}

type SpeechTaskStatusRequest struct {
	TaskID string `json:"task_id"`
}

type SpeechTaskMeta struct {
	DurationSeconds *float64 `json:"duration_seconds,omitempty"`
	SizeBytes       *int64   `json:"size_bytes,omitempty"`
	Format          string   `json:"format,omitempty"`
	SampleRate      *int     `json:"sample_rate,omitempty"`
	Bitrate         *int     `json:"bitrate,omitempty"`
	Channel         *int     `json:"channel,omitempty"`
}

type SpeechTaskResult struct {
	FileID      string         `json:"file_id,omitempty"`
	AudioURL    string         `json:"audio_url,omitempty"`
	RawHexAudio string         `json:"raw_hex_audio,omitempty"`
	Audio       []byte         `json:"audio,omitempty"`
	Meta        SpeechTaskMeta `json:"meta,omitempty"`
}

type SpeechTaskStatusResponse struct {
	TaskID       string                     `json:"task_id"`
	Status       SpeechTaskState            `json:"status,omitempty"`
	RawStatus    string                     `json:"raw_status,omitempty"`
	Result       SpeechTaskResult           `json:"result,omitempty"`
	ErrorMessage string                     `json:"error_message,omitempty"`
	Raw          map[string]json.RawMessage `json:"-"`
}

type speechAsyncSubmitWireRequest struct {
	Model        string              `json:"model"`
	Text         string              `json:"text,omitempty"`
	TextFileID   string              `json:"text_file_id,omitempty"`
	VoiceSetting *speechVoiceSetting `json:"voice_setting,omitempty"`
}

type speechAsyncSubmitRawResponse struct {
	TaskID          json.RawMessage              `json:"task_id,omitempty"`
	Status          string                       `json:"status,omitempty"`
	State           string                       `json:"state,omitempty"`
	FileID          json.RawMessage              `json:"file_id,omitempty"`
	TaskToken       string                       `json:"task_token,omitempty"`
	UsageCharacters *int64                       `json:"usage_characters,omitempty"`
	Data            *speechAsyncSubmitRawPayload `json:"data,omitempty"`
	Raw             map[string]json.RawMessage   `json:"-"`
}

type speechAsyncSubmitRawPayload struct {
	TaskID          json.RawMessage `json:"task_id,omitempty"`
	Status          string          `json:"status,omitempty"`
	State           string          `json:"state,omitempty"`
	FileID          json.RawMessage `json:"file_id,omitempty"`
	TaskToken       string          `json:"task_token,omitempty"`
	UsageCharacters *int64          `json:"usage_characters,omitempty"`
}

type speechTaskRawResponse struct {
	TaskID        json.RawMessage            `json:"task_id,omitempty"`
	Status        string                     `json:"status,omitempty"`
	State         string                     `json:"state,omitempty"`
	TaskState     string                     `json:"task_state,omitempty"`
	FileID        json.RawMessage            `json:"file_id,omitempty"`
	AudioURL      string                     `json:"audio_url,omitempty"`
	FileURL       string                     `json:"file_url,omitempty"`
	URL           string                     `json:"url,omitempty"`
	DownloadURL   string                     `json:"download_url,omitempty"`
	OutputURL     string                     `json:"output_url,omitempty"`
	AudioHex      string                     `json:"audio_hex,omitempty"`
	Audio         string                     `json:"audio,omitempty"`
	Hex           string                     `json:"hex,omitempty"`
	Error         string                     `json:"error,omitempty"`
	ErrorMsg      string                     `json:"error_msg,omitempty"`
	Message       string                     `json:"message,omitempty"`
	Duration      *float64                   `json:"duration,omitempty"`
	AudioLength   *float64                   `json:"audio_length,omitempty"`
	AudioDuration *float64                   `json:"audio_duration,omitempty"`
	Size          *int64                     `json:"size,omitempty"`
	Bytes         *int64                     `json:"bytes,omitempty"`
	FileSize      *int64                     `json:"file_size,omitempty"`
	AudioSize     *int64                     `json:"audio_size,omitempty"`
	Format        string                     `json:"format,omitempty"`
	SampleRate    *int                       `json:"sample_rate,omitempty"`
	Bitrate       *int                       `json:"bitrate,omitempty"`
	Channel       *int                       `json:"channel,omitempty"`
	Meta          *speechTaskMetaRaw         `json:"meta,omitempty"`
	ExtraInfo     *speechTaskMetaRaw         `json:"extra_info,omitempty"`
	AudioMeta     *speechTaskMetaRaw         `json:"audio_meta,omitempty"`
	AudioInfo     *speechTaskMetaRaw         `json:"audio_info,omitempty"`
	Data          *speechTaskRawPayload      `json:"data,omitempty"`
	Result        *speechTaskRawPayload      `json:"result,omitempty"`
	Output        *speechTaskRawPayload      `json:"output,omitempty"`
	Task          *speechTaskRawPayload      `json:"task,omitempty"`
	Raw           map[string]json.RawMessage `json:"-"`
}

type speechTaskRawPayload struct {
	TaskID        json.RawMessage       `json:"task_id,omitempty"`
	Status        string                `json:"status,omitempty"`
	State         string                `json:"state,omitempty"`
	TaskState     string                `json:"task_state,omitempty"`
	FileID        json.RawMessage       `json:"file_id,omitempty"`
	AudioURL      string                `json:"audio_url,omitempty"`
	FileURL       string                `json:"file_url,omitempty"`
	URL           string                `json:"url,omitempty"`
	DownloadURL   string                `json:"download_url,omitempty"`
	OutputURL     string                `json:"output_url,omitempty"`
	AudioHex      string                `json:"audio_hex,omitempty"`
	Audio         string                `json:"audio,omitempty"`
	Hex           string                `json:"hex,omitempty"`
	Error         string                `json:"error,omitempty"`
	ErrorMsg      string                `json:"error_msg,omitempty"`
	Message       string                `json:"message,omitempty"`
	Duration      *float64              `json:"duration,omitempty"`
	AudioLength   *float64              `json:"audio_length,omitempty"`
	AudioDuration *float64              `json:"audio_duration,omitempty"`
	Size          *int64                `json:"size,omitempty"`
	Bytes         *int64                `json:"bytes,omitempty"`
	FileSize      *int64                `json:"file_size,omitempty"`
	AudioSize     *int64                `json:"audio_size,omitempty"`
	Format        string                `json:"format,omitempty"`
	SampleRate    *int                  `json:"sample_rate,omitempty"`
	Bitrate       *int                  `json:"bitrate,omitempty"`
	Channel       *int                  `json:"channel,omitempty"`
	Meta          *speechTaskMetaRaw    `json:"meta,omitempty"`
	ExtraInfo     *speechTaskMetaRaw    `json:"extra_info,omitempty"`
	AudioMeta     *speechTaskMetaRaw    `json:"audio_meta,omitempty"`
	AudioInfo     *speechTaskMetaRaw    `json:"audio_info,omitempty"`
	Data          *speechTaskRawPayload `json:"data,omitempty"`
	Result        *speechTaskRawPayload `json:"result,omitempty"`
	Output        *speechTaskRawPayload `json:"output,omitempty"`
	Task          *speechTaskRawPayload `json:"task,omitempty"`
}

type speechTaskMetaRaw struct {
	Duration      *float64 `json:"duration,omitempty"`
	AudioLength   *float64 `json:"audio_length,omitempty"`
	AudioDuration *float64 `json:"audio_duration,omitempty"`
	Size          *int64   `json:"size,omitempty"`
	Bytes         *int64   `json:"bytes,omitempty"`
	FileSize      *int64   `json:"file_size,omitempty"`
	AudioSize     *int64   `json:"audio_size,omitempty"`
	Format        string   `json:"format,omitempty"`
	SampleRate    *int     `json:"sample_rate,omitempty"`
	Bitrate       *int     `json:"bitrate,omitempty"`
	Channel       *int     `json:"channel,omitempty"`
}

// SubmitAsync creates an asynchronous speech synthesis task.
func (s *SpeechAsyncService) SubmitAsync(ctx context.Context, request SpeechAsyncSubmitRequest) (*SpeechAsyncSubmitResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("speech async service is not initialized")
	}

	request.Model = strings.TrimSpace(request.Model)
	request.Text = strings.TrimSpace(request.Text)
	request.TextFileID = strings.TrimSpace(request.TextFileID)
	request.VoiceID = strings.TrimSpace(request.VoiceID)

	if request.Text == "" && request.TextFileID == "" {
		return nil, errors.New("speech async submit request text and text_file_id are empty")
	}

	if request.Model == "" {
		request.Model = defaultSpeechModel
	}

	wireRequest := speechAsyncSubmitWireRequest{
		Model:      request.Model,
		Text:       request.Text,
		TextFileID: request.TextFileID,
	}

	if request.VoiceID != "" || request.Speed != nil || request.Vol != nil || request.Pitch != nil {
		wireRequest.VoiceSetting = &speechVoiceSetting{
			VoiceID: request.VoiceID,
			Speed:   request.Speed,
			Vol:     request.Vol,
			Pitch:   request.Pitch,
		}
	}

	var rawResponse speechAsyncSubmitRawResponse
	if err := s.transport.DoJSON(ctx, transport.JSONRequest{
		Method: http.MethodPost,
		Path:   s.submitPath(),
		Body:   wireRequest,
	}, &rawResponse); err != nil {
		return nil, err
	}

	response := mapSpeechAsyncSubmitResponse(rawResponse)
	if response.TaskID == "" {
		return nil, errors.New("speech async submit response missing task_id")
	}

	if response.Status == "" {
		response.Status = SpeechTaskStateQueued
	}

	return response, nil
}

// GetAsyncTask queries a speech synthesis async task by task_id.
func (s *SpeechAsyncService) GetAsyncTask(ctx context.Context, taskID string) (*SpeechTaskStatusResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("speech async service is not initialized")
	}

	request := SpeechTaskStatusRequest{TaskID: strings.TrimSpace(taskID)}
	if request.TaskID == "" {
		return nil, errors.New("speech async query task_id is empty")
	}

	query := make(url.Values, 1)
	query.Set("task_id", request.TaskID)

	var rawResponse speechTaskRawResponse
	if err := s.transport.DoJSON(ctx, transport.JSONRequest{
		Method: http.MethodGet,
		Path:   s.queryPath(),
		Query:  query,
	}, &rawResponse); err != nil {
		return nil, err
	}

	response, err := mapSpeechTaskStatusResponse(rawResponse)
	if err != nil {
		return nil, err
	}

	if response.TaskID == "" {
		response.TaskID = request.TaskID
	}

	if response.Status == "" {
		if response.ErrorMessage != "" {
			response.Status = SpeechTaskStateFailed
		} else if response.Result.AudioURL != "" || response.Result.RawHexAudio != "" {
			response.Status = SpeechTaskStateSucceeded
		} else {
			response.Status = SpeechTaskStateRunning
		}
	}

	if response.ErrorMessage == "" && response.Status == SpeechTaskStateFailed {
		response.ErrorMessage = response.RawStatus
	}

	return response, nil
}

// SubmitAsync creates an asynchronous speech synthesis task through SpeechService.
func (s *SpeechService) SubmitAsync(ctx context.Context, request SpeechAsyncSubmitRequest) (*SpeechAsyncSubmitResponse, error) {
	return s.asSpeechAsyncService().SubmitAsync(ctx, request)
}

// GetAsyncTask queries an asynchronous speech synthesis task through SpeechService.
func (s *SpeechService) GetAsyncTask(ctx context.Context, taskID string) (*SpeechTaskStatusResponse, error) {
	return s.asSpeechAsyncService().GetAsyncTask(ctx, taskID)
}

func (s *SpeechService) asSpeechAsyncService() *SpeechAsyncService {
	if s == nil {
		return nil
	}

	return &SpeechAsyncService{
		transport:      s.transport,
		submitEndpoint: defaultSpeechAsyncSubmitPath,
		queryEndpoint:  defaultSpeechAsyncQueryPath,
	}
}

func (s *SpeechAsyncService) submitPath() string {
	if s == nil {
		return defaultSpeechAsyncSubmitPath
	}

	if endpoint := strings.TrimSpace(s.submitEndpoint); endpoint != "" {
		return endpoint
	}

	return defaultSpeechAsyncSubmitPath
}

func (s *SpeechAsyncService) queryPath() string {
	if s == nil {
		return defaultSpeechAsyncQueryPath
	}

	if endpoint := strings.TrimSpace(s.queryEndpoint); endpoint != "" {
		return endpoint
	}

	return defaultSpeechAsyncQueryPath
}

func mapSpeechAsyncSubmitResponse(raw speechAsyncSubmitRawResponse) *SpeechAsyncSubmitResponse {
	response := &SpeechAsyncSubmitResponse{
		TaskID: rawIDToString(raw.TaskID),
		Status: normalizeSpeechTaskState(firstNonEmpty(
			strings.TrimSpace(raw.Status),
			strings.TrimSpace(raw.State),
		)),
		RawStatus: strings.TrimSpace(firstNonEmpty(raw.Status, raw.State)),
		FileID:    rawIDToString(raw.FileID),
		TaskToken: strings.TrimSpace(raw.TaskToken),
		Raw:       cloneRawMessages(raw.Raw),
	}

	if raw.UsageCharacters != nil {
		response.UsageCharacters = cloneInt64Pointer(raw.UsageCharacters)
	}

	if raw.Data != nil {
		if response.TaskID == "" {
			response.TaskID = rawIDToString(raw.Data.TaskID)
		}
		if response.Status == "" {
			candidateStatus := strings.TrimSpace(firstNonEmpty(raw.Data.Status, raw.Data.State))
			if candidateStatus != "" {
				response.RawStatus = candidateStatus
				response.Status = normalizeSpeechTaskState(candidateStatus)
			}
		}
		if response.FileID == "" {
			response.FileID = rawIDToString(raw.Data.FileID)
		}
		if response.TaskToken == "" {
			response.TaskToken = strings.TrimSpace(raw.Data.TaskToken)
		}
		if response.UsageCharacters == nil {
			response.UsageCharacters = cloneInt64Pointer(raw.Data.UsageCharacters)
		}
	}

	return response
}

func mapSpeechTaskStatusResponse(raw speechTaskRawResponse) (*SpeechTaskStatusResponse, error) {
	payloads := collectSpeechTaskPayloads(raw.Data, raw.Result, raw.Output, raw.Task)

	taskID := rawIDToString(raw.TaskID)
	rawStatus := strings.TrimSpace(firstNonEmpty(raw.Status, raw.State, raw.TaskState))
	fileID := rawIDToString(raw.FileID)
	audioURL := strings.TrimSpace(firstNonEmpty(raw.AudioURL, raw.FileURL, raw.URL, raw.DownloadURL, raw.OutputURL))
	rawHexAudio := strings.TrimSpace(firstNonEmpty(raw.AudioHex, raw.Audio, raw.Hex))
	errorMessage := strings.TrimSpace(firstNonEmpty(raw.ErrorMsg, raw.Error, raw.Message))

	for _, payload := range payloads {
		if payload == nil {
			continue
		}

		if taskID == "" {
			taskID = rawIDToString(payload.TaskID)
		}
		if rawStatus == "" || normalizeSpeechTaskState(rawStatus) == "" {
			candidateStatus := strings.TrimSpace(firstNonEmpty(payload.Status, payload.State, payload.TaskState))
			if candidateStatus != "" {
				rawStatus = candidateStatus
			}
		}
		if fileID == "" {
			fileID = rawIDToString(payload.FileID)
		}
		if audioURL == "" {
			audioURL = strings.TrimSpace(firstNonEmpty(payload.AudioURL, payload.FileURL, payload.URL, payload.DownloadURL, payload.OutputURL))
		}
		if rawHexAudio == "" {
			rawHexAudio = strings.TrimSpace(firstNonEmpty(payload.AudioHex, payload.Audio, payload.Hex))
		}
		if errorMessage == "" {
			errorMessage = strings.TrimSpace(firstNonEmpty(payload.ErrorMsg, payload.Error, payload.Message))
		}
	}

	meta := mapSpeechTaskMeta(raw, payloads)
	result := SpeechTaskResult{
		FileID:      fileID,
		AudioURL:    audioURL,
		RawHexAudio: rawHexAudio,
		Meta:        meta,
	}

	if rawHexAudio != "" {
		audio, err := codec.DecodeHexAudio(rawHexAudio)
		if err != nil {
			return nil, fmt.Errorf("decode async speech task audio: %w", err)
		}
		result.Audio = audio
	}

	return &SpeechTaskStatusResponse{
		TaskID:       taskID,
		Status:       normalizeSpeechTaskState(rawStatus),
		RawStatus:    rawStatus,
		Result:       result,
		ErrorMessage: errorMessage,
		Raw:          cloneRawMessages(raw.Raw),
	}, nil
}

func mapSpeechTaskMeta(raw speechTaskRawResponse, payloads []*speechTaskRawPayload) SpeechTaskMeta {
	meta := SpeechTaskMeta{
		DurationSeconds: firstFloat64Pointer(raw.Duration, raw.AudioLength, raw.AudioDuration),
		SizeBytes:       firstInt64Pointer(raw.Size, raw.Bytes, raw.FileSize, raw.AudioSize),
		Format:          strings.TrimSpace(raw.Format),
		SampleRate:      firstIntPointer(raw.SampleRate),
		Bitrate:         firstIntPointer(raw.Bitrate),
		Channel:         firstIntPointer(raw.Channel),
	}

	mergeSpeechTaskMeta(&meta, raw.Meta)
	mergeSpeechTaskMeta(&meta, raw.ExtraInfo)
	mergeSpeechTaskMeta(&meta, raw.AudioMeta)
	mergeSpeechTaskMeta(&meta, raw.AudioInfo)

	for _, payload := range payloads {
		mergeSpeechTaskPayloadMeta(&meta, payload)
	}

	return meta
}

func mergeSpeechTaskPayloadMeta(meta *SpeechTaskMeta, payload *speechTaskRawPayload) {
	if meta == nil || payload == nil {
		return
	}

	if meta.DurationSeconds == nil {
		meta.DurationSeconds = firstFloat64Pointer(payload.Duration, payload.AudioLength, payload.AudioDuration)
	}
	if meta.SizeBytes == nil {
		meta.SizeBytes = firstInt64Pointer(payload.Size, payload.Bytes, payload.FileSize, payload.AudioSize)
	}
	if meta.Format == "" {
		meta.Format = strings.TrimSpace(payload.Format)
	}
	if meta.SampleRate == nil {
		meta.SampleRate = firstIntPointer(payload.SampleRate)
	}
	if meta.Bitrate == nil {
		meta.Bitrate = firstIntPointer(payload.Bitrate)
	}
	if meta.Channel == nil {
		meta.Channel = firstIntPointer(payload.Channel)
	}

	mergeSpeechTaskMeta(meta, payload.Meta)
	mergeSpeechTaskMeta(meta, payload.ExtraInfo)
	mergeSpeechTaskMeta(meta, payload.AudioMeta)
	mergeSpeechTaskMeta(meta, payload.AudioInfo)
}

func mergeSpeechTaskMeta(meta *SpeechTaskMeta, rawMeta *speechTaskMetaRaw) {
	if meta == nil || rawMeta == nil {
		return
	}

	if meta.DurationSeconds == nil {
		meta.DurationSeconds = firstFloat64Pointer(rawMeta.Duration, rawMeta.AudioLength, rawMeta.AudioDuration)
	}
	if meta.SizeBytes == nil {
		meta.SizeBytes = firstInt64Pointer(rawMeta.Size, rawMeta.Bytes, rawMeta.FileSize, rawMeta.AudioSize)
	}
	if meta.Format == "" {
		meta.Format = strings.TrimSpace(rawMeta.Format)
	}
	if meta.SampleRate == nil {
		meta.SampleRate = firstIntPointer(rawMeta.SampleRate)
	}
	if meta.Bitrate == nil {
		meta.Bitrate = firstIntPointer(rawMeta.Bitrate)
	}
	if meta.Channel == nil {
		meta.Channel = firstIntPointer(rawMeta.Channel)
	}
}

func collectSpeechTaskPayloads(initial ...*speechTaskRawPayload) []*speechTaskRawPayload {
	queue := make([]*speechTaskRawPayload, 0, len(initial))
	queue = append(queue, initial...)

	payloads := make([]*speechTaskRawPayload, 0, len(initial))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == nil {
			continue
		}

		payloads = append(payloads, current)
		queue = append(queue, current.Data, current.Result, current.Output, current.Task)
	}

	return payloads
}

func normalizeSpeechTaskState(status string) SpeechTaskState {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "" {
		return ""
	}

	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")

	switch normalized {
	case "queued", "queue", "pending", "submitted", "created", "waiting", "wait":
		return SpeechTaskStateQueued
	case "running", "run", "processing", "in_progress", "progress", "working", "started":
		return SpeechTaskStateRunning
	case "succeeded", "success", "successful", "completed", "complete", "done", "finished", "finish":
		return SpeechTaskStateSucceeded
	case "failed", "fail", "error", "errored", "canceled", "cancelled", "aborted", "expired", "timeout", "timed_out", "rejected":
		return SpeechTaskStateFailed
	}

	switch {
	case strings.Contains(normalized, "succeed"), strings.Contains(normalized, "success"), strings.Contains(normalized, "complete"), strings.Contains(normalized, "finish"), strings.Contains(normalized, "done"):
		return SpeechTaskStateSucceeded
	case strings.Contains(normalized, "fail"), strings.Contains(normalized, "error"), strings.Contains(normalized, "cancel"), strings.Contains(normalized, "abort"), strings.Contains(normalized, "expire"), strings.Contains(normalized, "timeout"), strings.Contains(normalized, "reject"):
		return SpeechTaskStateFailed
	case strings.Contains(normalized, "process"), strings.Contains(normalized, "running"), strings.Contains(normalized, "progress"), strings.Contains(normalized, "start"):
		return SpeechTaskStateRunning
	case strings.Contains(normalized, "queue"), strings.Contains(normalized, "pend"), strings.Contains(normalized, "wait"), strings.Contains(normalized, "submit"), strings.Contains(normalized, "create"):
		return SpeechTaskStateQueued
	default:
		return ""
	}
}

func rawIDToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var textValue string
	if err := json.Unmarshal(raw, &textValue); err == nil {
		return strings.TrimSpace(textValue)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var numberValue json.Number
	if err := decoder.Decode(&numberValue); err == nil {
		return strings.TrimSpace(numberValue.String())
	}

	trimmed := strings.TrimSpace(string(raw))
	trimmed = strings.Trim(trimmed, `"`)
	if strings.EqualFold(trimmed, "null") {
		return ""
	}

	return strings.TrimSpace(trimmed)
}

func firstFloat64Pointer(values ...*float64) *float64 {
	for _, value := range values {
		if value != nil {
			copied := *value
			return &copied
		}
	}

	return nil
}

func firstInt64Pointer(values ...*int64) *int64 {
	for _, value := range values {
		if value != nil {
			copied := *value
			return &copied
		}
	}

	return nil
}

func firstIntPointer(values ...*int) *int {
	for _, value := range values {
		if value != nil {
			copied := *value
			return &copied
		}
	}

	return nil
}

func cloneInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}

	copied := *value
	return &copied
}

func (r *speechAsyncSubmitRawResponse) UnmarshalJSON(data []byte) error {
	type alias speechAsyncSubmitRawResponse

	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	delete(raw, "task_id")
	delete(raw, "status")
	delete(raw, "state")
	delete(raw, "file_id")
	delete(raw, "task_token")
	delete(raw, "usage_characters")
	delete(raw, "base_resp")
	delete(raw, "status_code")
	delete(raw, "status_msg")

	*r = speechAsyncSubmitRawResponse(parsed)
	if len(raw) > 0 {
		r.Raw = raw
	} else {
		r.Raw = nil
	}

	return nil
}

func (r *speechTaskRawResponse) UnmarshalJSON(data []byte) error {
	type alias speechTaskRawResponse

	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	delete(raw, "task_id")
	delete(raw, "status")
	delete(raw, "state")
	delete(raw, "task_state")
	delete(raw, "file_id")
	delete(raw, "audio_url")
	delete(raw, "file_url")
	delete(raw, "url")
	delete(raw, "download_url")
	delete(raw, "output_url")
	delete(raw, "audio_hex")
	delete(raw, "audio")
	delete(raw, "hex")
	delete(raw, "error")
	delete(raw, "error_msg")
	delete(raw, "message")
	delete(raw, "duration")
	delete(raw, "audio_length")
	delete(raw, "audio_duration")
	delete(raw, "size")
	delete(raw, "bytes")
	delete(raw, "file_size")
	delete(raw, "audio_size")
	delete(raw, "format")
	delete(raw, "sample_rate")
	delete(raw, "bitrate")
	delete(raw, "channel")
	delete(raw, "meta")
	delete(raw, "extra_info")
	delete(raw, "audio_meta")
	delete(raw, "audio_info")
	delete(raw, "base_resp")
	delete(raw, "status_code")
	delete(raw, "status_msg")

	*r = speechTaskRawResponse(parsed)
	if len(raw) > 0 {
		r.Raw = raw
	} else {
		r.Raw = nil
	}

	return nil
}
