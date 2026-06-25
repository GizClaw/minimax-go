package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
)

const maxBodyPreviewSize = 512

// BaseResp represents business status metadata in Minimax responses.
type BaseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

// APIError is the unified error model for HTTP and base_resp semantics.
type APIError struct {
	HTTPStatus int
	StatusCode int
	StatusMsg  string
	RequestID  string
	TraceID    string
	Body       string
	Cause      error
}

func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}

	if e.StatusCode != 0 {
		return fmt.Sprintf("minimax api error: status_code=%d status_msg=%q", e.StatusCode, e.StatusMsg)
	}

	if e.HTTPStatus != 0 {
		if e.Body != "" {
			return fmt.Sprintf("minimax http error: status=%d body=%q", e.HTTPStatus, e.Body)
		}
		return fmt.Sprintf("minimax http error: status=%d", e.HTTPStatus)
	}

	if e.Cause != nil {
		return fmt.Sprintf("minimax transport error: %v", e.Cause)
	}

	return "minimax api error"
}

func (e *APIError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewHTTPError(httpStatus int, body []byte) *APIError {
	traceMeta := ExtractTraceMeta(body)
	return &APIError{
		HTTPStatus: httpStatus,
		StatusMsg:  http.StatusText(httpStatus),
		RequestID:  traceMeta.RequestID,
		TraceID:    traceMeta.TraceID,
		Body:       compactBody(body),
	}
}

func NewBaseRespError(httpStatus int, baseResp BaseResp, body []byte) *APIError {
	traceMeta := ExtractTraceMeta(body)
	return &APIError{
		HTTPStatus: httpStatus,
		StatusCode: baseResp.StatusCode,
		StatusMsg:  baseResp.StatusMsg,
		RequestID:  traceMeta.RequestID,
		TraceID:    traceMeta.TraceID,
		Body:       compactBody(body),
	}
}

type TraceMeta struct {
	RequestID string
	TraceID   string
}

type responseEnvelope struct {
	BaseResp   *BaseResp `json:"base_resp"`
	StatusCode *int      `json:"status_code"`
	StatusMsg  *string   `json:"status_msg"`
}

type traceEnvelope struct {
	RequestID string         `json:"request_id"`
	TraceID   string         `json:"trace_id"`
	LogID     string         `json:"log_id"`
	BaseResp  *traceEnvelope `json:"base_resp"`
}

// ParseBaseResp extracts base_resp fields from a response body.
func ParseBaseResp(body []byte) (BaseResp, bool) {
	if len(body) == 0 {
		return BaseResp{}, false
	}

	var envelope responseEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return BaseResp{}, false
	}

	if envelope.BaseResp != nil {
		return *envelope.BaseResp, true
	}

	if envelope.StatusCode != nil || envelope.StatusMsg != nil {
		resp := BaseResp{}
		if envelope.StatusCode != nil {
			resp.StatusCode = *envelope.StatusCode
		}
		if envelope.StatusMsg != nil {
			resp.StatusMsg = *envelope.StatusMsg
		}
		return resp, true
	}

	return BaseResp{}, false
}

// CheckResponse normalizes HTTP and business status into a unified error.
func CheckResponse(httpStatus int, body []byte) error {
	return CheckResponseWithTrace(httpStatus, body, TraceMeta{})
}

// CheckResponseWithTrace normalizes HTTP and business status with request trace metadata.
func CheckResponseWithTrace(httpStatus int, body []byte, traceMeta TraceMeta) error {
	traceMeta = mergeTraceMeta(traceMeta, ExtractTraceMeta(body))

	if httpStatus < http.StatusOK || httpStatus >= http.StatusMultipleChoices {
		err := NewHTTPError(httpStatus, body)
		applyTraceMeta(err, traceMeta)
		return err
	}

	if baseResp, ok := ParseBaseResp(body); ok && baseResp.StatusCode != 0 {
		err := NewBaseRespError(httpStatus, baseResp, body)
		applyTraceMeta(err, traceMeta)
		return err
	}

	return nil
}

// ExtractTraceMeta extracts request and trace identifiers from response JSON bodies.
func ExtractTraceMeta(body []byte) TraceMeta {
	if len(body) == 0 {
		return TraceMeta{}
	}

	var envelope traceEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return TraceMeta{}
	}

	meta := TraceMeta{
		RequestID: strings.TrimSpace(envelope.RequestID),
		TraceID:   firstNonEmpty(strings.TrimSpace(envelope.TraceID), strings.TrimSpace(envelope.LogID)),
	}
	if envelope.BaseResp != nil {
		meta = mergeTraceMeta(meta, TraceMeta{
			RequestID: strings.TrimSpace(envelope.BaseResp.RequestID),
			TraceID:   firstNonEmpty(strings.TrimSpace(envelope.BaseResp.TraceID), strings.TrimSpace(envelope.BaseResp.LogID)),
		})
	}

	return meta
}

// IsRetryable reports whether an error is retryable.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	if apiErr, ok := errors.AsType[*APIError](err); ok {
		return apiErr.HTTPStatus == http.StatusTooManyRequests || apiErr.HTTPStatus >= http.StatusInternalServerError
	}

	if netErr, ok := errors.AsType[net.Error](err); ok {
		return netErr.Timeout()
	}

	return false
}

func compactBody(body []byte) string {
	s := strings.TrimSpace(string(body))
	if len(s) <= maxBodyPreviewSize {
		return s
	}
	return s[:maxBodyPreviewSize] + "..."
}

func applyTraceMeta(err *APIError, traceMeta TraceMeta) {
	if err == nil {
		return
	}

	err.RequestID = firstNonEmpty(traceMeta.RequestID, err.RequestID)
	err.TraceID = firstNonEmpty(traceMeta.TraceID, err.TraceID)
}

func mergeTraceMeta(primary, fallback TraceMeta) TraceMeta {
	return TraceMeta{
		RequestID: firstNonEmpty(primary.RequestID, fallback.RequestID),
		TraceID:   firstNonEmpty(primary.TraceID, fallback.TraceID),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
