package minimax

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// VideoImageToVideoRequest contains parameters for MiniMax image-to-video task creation.
type VideoImageToVideoRequest struct {
	Model            string `json:"model"`
	FirstFrameImage  string `json:"first_frame_image"`
	Prompt           string `json:"prompt,omitempty"`
	PromptOptimizer  *bool  `json:"prompt_optimizer,omitempty"`
	FastPretreatment *bool  `json:"fast_pretreatment,omitempty"`
	Duration         *int   `json:"duration,omitempty"`
	Resolution       string `json:"resolution,omitempty"`
	CallbackURL      string `json:"callback_url,omitempty"`
	AIGCWatermark    *bool  `json:"aigc_watermark,omitempty"`
}

// VideoFirstLastFrameRequest contains parameters for MiniMax first-last-frame video task creation.
type VideoFirstLastFrameRequest struct {
	Model           string `json:"model"`
	LastFrameImage  string `json:"last_frame_image"`
	FirstFrameImage string `json:"first_frame_image,omitempty"`
	Prompt          string `json:"prompt,omitempty"`
	PromptOptimizer *bool  `json:"prompt_optimizer,omitempty"`
	Duration        *int   `json:"duration,omitempty"`
	Resolution      string `json:"resolution,omitempty"`
	CallbackURL     string `json:"callback_url,omitempty"`
	AIGCWatermark   *bool  `json:"aigc_watermark,omitempty"`
}

// VideoSubjectReferenceRequest contains parameters for MiniMax subject-reference video task creation.
type VideoSubjectReferenceRequest struct {
	Model             string                  `json:"model"`
	SubjectReferences []VideoSubjectReference `json:"subject_reference"`
	Prompt            string                  `json:"prompt,omitempty"`
	PromptOptimizer   *bool                   `json:"prompt_optimizer,omitempty"`
	CallbackURL       string                  `json:"callback_url,omitempty"`
	AIGCWatermark     *bool                   `json:"aigc_watermark,omitempty"`
}

// VideoSubjectReference describes one subject reference for subject-reference video generation.
type VideoSubjectReference struct {
	Type  string   `json:"type"`
	Image []string `json:"image"`
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

	normalizeVideoTextToVideoRequest(&request)
	if err := validateVideoTextToVideoRequest(request); err != nil {
		return nil, err
	}

	return s.createTask(ctx, request, "video text-to-video")
}

// CreateImageToVideo creates an async image-to-video generation task.
func (s *VideoService) CreateImageToVideo(ctx context.Context, request VideoImageToVideoRequest) (*VideoTaskCreateResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("video service is not initialized")
	}

	normalizeVideoImageToVideoRequest(&request)
	if err := validateVideoImageToVideoRequest(request); err != nil {
		return nil, err
	}

	return s.createTask(ctx, request, "video image-to-video")
}

// CreateFirstLastFrameVideo creates an async first-last-frame video generation task.
func (s *VideoService) CreateFirstLastFrameVideo(ctx context.Context, request VideoFirstLastFrameRequest) (*VideoTaskCreateResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("video service is not initialized")
	}

	normalizeVideoFirstLastFrameRequest(&request)
	if err := validateVideoFirstLastFrameRequest(request); err != nil {
		return nil, err
	}

	return s.createTask(ctx, request, "video first-last-frame")
}

// CreateSubjectReferenceVideo creates an async subject-reference video generation task.
func (s *VideoService) CreateSubjectReferenceVideo(ctx context.Context, request VideoSubjectReferenceRequest) (*VideoTaskCreateResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("video service is not initialized")
	}

	normalizeVideoSubjectReferenceRequest(&request)
	if err := validateVideoSubjectReferenceRequest(request); err != nil {
		return nil, err
	}

	return s.createTask(ctx, request, "video subject-reference")
}

func (s *VideoService) createTask(ctx context.Context, request any, prefix string) (*VideoTaskCreateResponse, error) {
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
		return nil, errors.New(prefix + " response missing task_id")
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

func normalizeVideoTextToVideoRequest(request *VideoTextToVideoRequest) {
	request.Model = strings.TrimSpace(request.Model)
	request.Prompt = strings.TrimSpace(request.Prompt)
	request.Resolution = strings.TrimSpace(request.Resolution)
	request.CallbackURL = strings.TrimSpace(request.CallbackURL)
}

func normalizeVideoImageToVideoRequest(request *VideoImageToVideoRequest) {
	request.Model = strings.TrimSpace(request.Model)
	request.FirstFrameImage = strings.TrimSpace(request.FirstFrameImage)
	request.Prompt = strings.TrimSpace(request.Prompt)
	request.Resolution = strings.TrimSpace(request.Resolution)
	request.CallbackURL = strings.TrimSpace(request.CallbackURL)
}

func normalizeVideoFirstLastFrameRequest(request *VideoFirstLastFrameRequest) {
	request.Model = strings.TrimSpace(request.Model)
	request.LastFrameImage = strings.TrimSpace(request.LastFrameImage)
	request.FirstFrameImage = strings.TrimSpace(request.FirstFrameImage)
	request.Prompt = strings.TrimSpace(request.Prompt)
	request.Resolution = strings.TrimSpace(request.Resolution)
	request.CallbackURL = strings.TrimSpace(request.CallbackURL)
}

func normalizeVideoSubjectReferenceRequest(request *VideoSubjectReferenceRequest) {
	request.Model = strings.TrimSpace(request.Model)
	request.Prompt = strings.TrimSpace(request.Prompt)
	request.CallbackURL = strings.TrimSpace(request.CallbackURL)
	for referenceIndex := range request.SubjectReferences {
		request.SubjectReferences[referenceIndex].Type = strings.TrimSpace(request.SubjectReferences[referenceIndex].Type)
		for imageIndex := range request.SubjectReferences[referenceIndex].Image {
			request.SubjectReferences[referenceIndex].Image[imageIndex] = strings.TrimSpace(request.SubjectReferences[referenceIndex].Image[imageIndex])
		}
	}
}

func validateVideoTextToVideoRequest(request VideoTextToVideoRequest) error {
	if request.Model == "" {
		return errors.New("video text-to-video request model is empty")
	}
	if request.Prompt == "" {
		return errors.New("video text-to-video request prompt is empty")
	}

	return nil
}

func validateVideoImageToVideoRequest(request VideoImageToVideoRequest) error {
	if request.Model == "" {
		return errors.New("video image-to-video request model is empty")
	}
	if request.FirstFrameImage == "" {
		return errors.New("video image-to-video request first_frame_image is empty")
	}

	return nil
}

func validateVideoFirstLastFrameRequest(request VideoFirstLastFrameRequest) error {
	if request.Model == "" {
		return errors.New("video first-last-frame request model is empty")
	}
	if request.LastFrameImage == "" {
		return errors.New("video first-last-frame request last_frame_image is empty")
	}

	return nil
}

func validateVideoSubjectReferenceRequest(request VideoSubjectReferenceRequest) error {
	if request.Model == "" {
		return errors.New("video subject-reference request model is empty")
	}
	if len(request.SubjectReferences) == 0 {
		return errors.New("video subject-reference request subject_reference is empty")
	}
	for referenceIndex, reference := range request.SubjectReferences {
		if reference.Type == "" {
			return fmt.Errorf("video subject-reference request subject_reference[%d].type is empty", referenceIndex)
		}
		if len(reference.Image) == 0 {
			return fmt.Errorf("video subject-reference request subject_reference[%d].image is empty", referenceIndex)
		}
		for imageIndex, image := range reference.Image {
			if image == "" {
				return fmt.Errorf("video subject-reference request subject_reference[%d].image[%d] is empty", referenceIndex, imageIndex)
			}
		}
	}

	return nil
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
