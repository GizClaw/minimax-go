package minimax

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/GizClaw/minimax-go/internal/transport"
)

const (
	defaultVideoGenerationPath = "/v1/video_generation"
	defaultVideoQueryPath      = "/v1/query/video_generation"
)

// VideoTaskState is the normalized video generation task state.
type VideoTaskState string

const (
	VideoTaskStateProcessing VideoTaskState = "processing"
	VideoTaskStateSucceeded  VideoTaskState = "success"
	VideoTaskStateFailed     VideoTaskState = "failed"
)

// IsTerminal reports whether the task state is terminal.
func (s VideoTaskState) IsTerminal() bool {
	return s == VideoTaskStateSucceeded || s == VideoTaskStateFailed
}

type VideoService struct {
	transport      *transport.Client
	createEndpoint string
	queryEndpoint  string
}

type VideoTextToVideoRequest struct {
	Model            string `json:"model"`
	Prompt           string `json:"prompt"`
	PromptOptimizer  *bool  `json:"prompt_optimizer,omitempty"`
	FastPretreatment *bool  `json:"fast_pretreatment,omitempty"`
	Duration         *int   `json:"duration,omitempty"`
	Resolution       string `json:"resolution,omitempty"`
	CallbackURL      string `json:"callback_url,omitempty"`
	AIGCWatermark    *bool  `json:"aigc_watermark,omitempty"`
}

type VideoTaskCreateResponse struct {
	ResponseMeta ResponseMeta               `json:"response_meta,omitzero"`
	TaskID       string                     `json:"task_id"`
	Raw          map[string]json.RawMessage `json:"-"`
}

type VideoTaskStatusResponse struct {
	ResponseMeta ResponseMeta               `json:"response_meta,omitzero"`
	TaskID       string                     `json:"task_id"`
	Status       VideoTaskState             `json:"status,omitempty"`
	RawStatus    string                     `json:"raw_status,omitempty"`
	FileID       string                     `json:"file_id,omitempty"`
	VideoWidth   *int                       `json:"video_width,omitempty"`
	VideoHeight  *int                       `json:"video_height,omitempty"`
	FailureCode  string                     `json:"failure_code,omitempty"`
	FailureMsg   string                     `json:"failure_msg,omitempty"`
	Raw          map[string]json.RawMessage `json:"-"`
}

type videoTaskCreateRawResponse struct {
	TaskID json.RawMessage            `json:"task_id,omitempty"`
	Data   *videoTaskCreateRawData    `json:"data,omitempty"`
	Raw    map[string]json.RawMessage `json:"-"`
}

type videoTaskCreateRawData struct {
	TaskID json.RawMessage `json:"task_id,omitempty"`
}

type videoTaskStatusRawResponse struct {
	TaskID      json.RawMessage            `json:"task_id,omitempty"`
	Status      string                     `json:"status,omitempty"`
	State       string                     `json:"state,omitempty"`
	TaskState   string                     `json:"task_state,omitempty"`
	FileID      json.RawMessage            `json:"file_id,omitempty"`
	VideoWidth  *int                       `json:"video_width,omitempty"`
	VideoHeight *int                       `json:"video_height,omitempty"`
	FailureCode json.RawMessage            `json:"failure_code,omitempty"`
	FailureMsg  string                     `json:"failure_msg,omitempty"`
	ErrorCode   json.RawMessage            `json:"error_code,omitempty"`
	ErrorMsg    string                     `json:"error_msg,omitempty"`
	Error       string                     `json:"error,omitempty"`
	Message     string                     `json:"message,omitempty"`
	Data        *videoTaskStatusRawPayload `json:"data,omitempty"`
	Result      *videoTaskStatusRawPayload `json:"result,omitempty"`
	Task        *videoTaskStatusRawPayload `json:"task,omitempty"`
	Raw         map[string]json.RawMessage `json:"-"`
}

type videoTaskStatusRawPayload struct {
	TaskID      json.RawMessage            `json:"task_id,omitempty"`
	Status      string                     `json:"status,omitempty"`
	State       string                     `json:"state,omitempty"`
	TaskState   string                     `json:"task_state,omitempty"`
	FileID      json.RawMessage            `json:"file_id,omitempty"`
	VideoWidth  *int                       `json:"video_width,omitempty"`
	VideoHeight *int                       `json:"video_height,omitempty"`
	FailureCode json.RawMessage            `json:"failure_code,omitempty"`
	FailureMsg  string                     `json:"failure_msg,omitempty"`
	ErrorCode   json.RawMessage            `json:"error_code,omitempty"`
	ErrorMsg    string                     `json:"error_msg,omitempty"`
	Error       string                     `json:"error,omitempty"`
	Message     string                     `json:"message,omitempty"`
	Data        *videoTaskStatusRawPayload `json:"data,omitempty"`
	Result      *videoTaskStatusRawPayload `json:"result,omitempty"`
	Task        *videoTaskStatusRawPayload `json:"task,omitempty"`
}

// CreateTextToVideo creates an async text-to-video generation task.
func (s *VideoService) CreateTextToVideo(ctx context.Context, request VideoTextToVideoRequest) (*VideoTaskCreateResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("video service is not initialized")
	}

	request.Model = strings.TrimSpace(request.Model)
	request.Prompt = strings.TrimSpace(request.Prompt)
	request.Resolution = strings.TrimSpace(request.Resolution)
	request.CallbackURL = strings.TrimSpace(request.CallbackURL)

	if request.Model == "" {
		return nil, errors.New("video text-to-video request model is empty")
	}
	if request.Prompt == "" {
		return nil, errors.New("video text-to-video request prompt is empty")
	}

	var raw videoTaskCreateRawResponse
	meta, err := s.transport.DoJSONWithMeta(ctx, transport.JSONRequest{
		Method: http.MethodPost,
		Path:   s.resolveCreatePath(),
		Body:   request,
	}, &raw)
	if err != nil {
		return nil, err
	}

	response := mapVideoTaskCreateResponse(raw)
	response.ResponseMeta = responseMetaFromTransport(meta)
	if response.TaskID == "" {
		return nil, errors.New("video text-to-video response missing task_id")
	}

	return response, nil
}

// GetTask queries an async video generation task.
func (s *VideoService) GetTask(ctx context.Context, taskID string) (*VideoTaskStatusResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("video service is not initialized")
	}

	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, errors.New("video task query task_id is empty")
	}

	query := url.Values{}
	query.Set("task_id", taskID)

	var raw videoTaskStatusRawResponse
	meta, err := s.transport.DoJSONWithMeta(ctx, transport.JSONRequest{
		Method: http.MethodGet,
		Path:   s.resolveQueryPath(),
		Query:  query,
	}, &raw)
	if err != nil {
		return nil, err
	}

	response := mapVideoTaskStatusResponse(raw, taskID)
	response.ResponseMeta = responseMetaFromTransport(meta)
	return response, nil
}

func (s *VideoService) resolveCreatePath() string {
	createPath := strings.TrimSpace(s.createEndpoint)
	if createPath != "" {
		return createPath
	}

	return defaultVideoGenerationPath
}

func (s *VideoService) resolveQueryPath() string {
	queryPath := strings.TrimSpace(s.queryEndpoint)
	if queryPath != "" {
		return queryPath
	}

	return defaultVideoQueryPath
}

func mapVideoTaskCreateResponse(raw videoTaskCreateRawResponse) *VideoTaskCreateResponse {
	taskID := rawIDToString(raw.TaskID)
	if raw.Data != nil {
		taskID = firstNonEmptyValue(taskID, rawIDToString(raw.Data.TaskID))
	}

	return &VideoTaskCreateResponse{
		TaskID: taskID,
		Raw:    cloneRawMessages(raw.Raw),
	}
}

func mapVideoTaskStatusResponse(raw videoTaskStatusRawResponse, fallbackTaskID string) *VideoTaskStatusResponse {
	response := &VideoTaskStatusResponse{
		TaskID:      firstNonEmptyValue(rawIDToString(raw.TaskID), fallbackTaskID),
		RawStatus:   firstNonEmptyValue(raw.Status, raw.State, raw.TaskState),
		FileID:      rawIDToString(raw.FileID),
		VideoWidth:  firstIntPointer(raw.VideoWidth),
		VideoHeight: firstIntPointer(raw.VideoHeight),
		FailureCode: firstNonEmptyValue(rawIDToString(raw.FailureCode), rawIDToString(raw.ErrorCode)),
		FailureMsg:  firstNonEmptyValue(raw.FailureMsg, raw.ErrorMsg, raw.Error, raw.Message),
		Raw:         cloneRawMessages(raw.Raw),
	}

	for _, payload := range flattenVideoTaskPayloads(raw.Data, raw.Result, raw.Task) {
		response.TaskID = firstNonEmptyValue(rawIDToString(payload.TaskID), response.TaskID)
		response.RawStatus = firstNonEmptyValue(response.RawStatus, payload.Status, payload.State, payload.TaskState)
		response.FileID = firstNonEmptyValue(response.FileID, rawIDToString(payload.FileID))
		response.VideoWidth = firstIntPointer(response.VideoWidth, payload.VideoWidth)
		response.VideoHeight = firstIntPointer(response.VideoHeight, payload.VideoHeight)
		response.FailureCode = firstNonEmptyValue(response.FailureCode, rawIDToString(payload.FailureCode), rawIDToString(payload.ErrorCode))
		response.FailureMsg = firstNonEmptyValue(response.FailureMsg, payload.FailureMsg, payload.ErrorMsg, payload.Error, payload.Message)
	}

	response.Status = normalizeVideoTaskState(response.RawStatus)
	return response
}

func flattenVideoTaskPayloads(payloads ...*videoTaskStatusRawPayload) []*videoTaskStatusRawPayload {
	var flattened []*videoTaskStatusRawPayload
	queue := append([]*videoTaskStatusRawPayload(nil), payloads...)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == nil {
			continue
		}

		flattened = append(flattened, current)
		queue = append(queue, current.Data, current.Result, current.Task)
	}

	return flattened
}

func normalizeVideoTaskState(status string) VideoTaskState {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "" {
		return ""
	}

	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")

	switch normalized {
	case "preparing", "prepare", "queueing", "queued", "queue", "processing", "process", "pending", "running", "in_progress":
		return VideoTaskStateProcessing
	case "success", "succeeded", "successful", "complete", "completed", "done", "finished":
		return VideoTaskStateSucceeded
	case "fail", "failed", "failure", "error", "errored", "canceled", "cancelled", "aborted", "expired", "timeout", "timed_out", "rejected":
		return VideoTaskStateFailed
	}

	switch {
	case strings.Contains(normalized, "success"), strings.Contains(normalized, "complete"), strings.Contains(normalized, "done"), strings.Contains(normalized, "finish"):
		return VideoTaskStateSucceeded
	case strings.Contains(normalized, "fail"), strings.Contains(normalized, "error"), strings.Contains(normalized, "cancel"), strings.Contains(normalized, "abort"), strings.Contains(normalized, "expire"), strings.Contains(normalized, "timeout"), strings.Contains(normalized, "reject"):
		return VideoTaskStateFailed
	case strings.Contains(normalized, "prepar"), strings.Contains(normalized, "queue"), strings.Contains(normalized, "process"), strings.Contains(normalized, "pend"), strings.Contains(normalized, "running"), strings.Contains(normalized, "progress"):
		return VideoTaskStateProcessing
	default:
		return ""
	}
}

func (r *videoTaskCreateRawResponse) UnmarshalJSON(data []byte) error {
	type alias videoTaskCreateRawResponse
	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	delete(raw, "task_id")
	delete(raw, "data")
	delete(raw, "base_resp")
	delete(raw, "status_code")
	delete(raw, "status_msg")

	*r = videoTaskCreateRawResponse(parsed)
	if len(raw) > 0 {
		r.Raw = raw
	} else {
		r.Raw = nil
	}

	return nil
}

func (r *videoTaskStatusRawResponse) UnmarshalJSON(data []byte) error {
	type alias videoTaskStatusRawResponse
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
	delete(raw, "video_width")
	delete(raw, "video_height")
	delete(raw, "failure_code")
	delete(raw, "failure_msg")
	delete(raw, "error_code")
	delete(raw, "error_msg")
	delete(raw, "error")
	delete(raw, "message")
	delete(raw, "data")
	delete(raw, "result")
	delete(raw, "task")
	delete(raw, "base_resp")
	delete(raw, "status_code")
	delete(raw, "status_msg")

	*r = videoTaskStatusRawResponse(parsed)
	if len(raw) > 0 {
		r.Raw = raw
	} else {
		r.Raw = nil
	}

	return nil
}
