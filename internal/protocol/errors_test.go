package protocol

import (
	"context"
	"errors"
	"testing"
)

func TestParseBaseResp(t *testing.T) {
	t.Parallel()

	t.Run("nested base_resp", func(t *testing.T) {
		t.Parallel()

		body := []byte(`{"base_resp":{"status_code":1001,"status_msg":"bad request"}}`)
		resp, ok := ParseBaseResp(body)
		if !ok {
			t.Fatalf("ParseBaseResp() ok = false, want true")
		}

		if resp.StatusCode != 1001 || resp.StatusMsg != "bad request" {
			t.Fatalf("ParseBaseResp() = %+v, want status_code=1001 status_msg=bad request", resp)
		}
	})

	t.Run("top level status", func(t *testing.T) {
		t.Parallel()

		body := []byte(`{"status_code":42,"status_msg":"failed"}`)
		resp, ok := ParseBaseResp(body)
		if !ok {
			t.Fatalf("ParseBaseResp() ok = false, want true")
		}

		if resp.StatusCode != 42 || resp.StatusMsg != "failed" {
			t.Fatalf("ParseBaseResp() = %+v, want status_code=42 status_msg=failed", resp)
		}
	})
}

func TestCheckResponse(t *testing.T) {
	t.Parallel()

	t.Run("http error", func(t *testing.T) {
		t.Parallel()

		err := CheckResponse(500, []byte(`{"error":"boom"}`))
		if err == nil {
			t.Fatal("CheckResponse() error = nil, want non-nil")
		}

		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("CheckResponse() error type = %T, want *APIError", err)
		}

		if apiErr.HTTPStatus != 500 {
			t.Fatalf("HTTPStatus = %d, want 500", apiErr.HTTPStatus)
		}
	})

	t.Run("base_resp error", func(t *testing.T) {
		t.Parallel()

		err := CheckResponse(200, []byte(`{"base_resp":{"status_code":2301,"status_msg":"invalid voice"}}`))
		if err == nil {
			t.Fatal("CheckResponse() error = nil, want non-nil")
		}

		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("CheckResponse() error type = %T, want *APIError", err)
		}

		if apiErr.StatusCode != 2301 || apiErr.StatusMsg != "invalid voice" {
			t.Fatalf("apiErr = %+v, want status_code=2301 status_msg=invalid voice", apiErr)
		}
	})
}

func TestTraceMeta(t *testing.T) {
	t.Parallel()

	t.Run("extracts top level trace identifiers", func(t *testing.T) {
		t.Parallel()

		meta := ExtractTraceMeta([]byte(`{"request_id":"req-1","trace_id":"trace-1"}`))
		if meta.RequestID != "req-1" || meta.TraceID != "trace-1" {
			t.Fatalf("ExtractTraceMeta() = %+v, want request_id=req-1 trace_id=trace-1", meta)
		}
	})

	t.Run("base_resp error carries trace identifiers", func(t *testing.T) {
		t.Parallel()

		err := CheckResponse(200, []byte(`{"base_resp":{"status_code":2301,"status_msg":"invalid voice","request_id":"req-2","trace_id":"trace-2"}}`))
		if err == nil {
			t.Fatal("CheckResponse() error = nil, want non-nil")
		}

		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("CheckResponse() error type = %T, want *APIError", err)
		}

		if apiErr.RequestID != "req-2" || apiErr.TraceID != "trace-2" {
			t.Fatalf("apiErr trace = request_id:%q trace_id:%q, want req-2 trace-2", apiErr.RequestID, apiErr.TraceID)
		}
	})
}

func TestIsRetryable(t *testing.T) {
	t.Parallel()

	if !IsRetryable(NewHTTPError(429, nil)) {
		t.Fatal("IsRetryable(429) = false, want true")
	}

	if !IsRetryable(NewHTTPError(503, nil)) {
		t.Fatal("IsRetryable(503) = false, want true")
	}

	if IsRetryable(context.Canceled) {
		t.Fatal("IsRetryable(context.Canceled) = true, want false")
	}

	if IsRetryable(context.DeadlineExceeded) {
		t.Fatal("IsRetryable(context.DeadlineExceeded) = true, want false")
	}
}
