package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/GizClaw/minimax-go/internal/protocol"
)

// OpenUploadWithMeta sends a replayable multipart upload. Streaming bodies belong
// to the caller; retries stop once a stream opens, so deltas are never duplicated.
func (c *Client) OpenUploadWithMeta(ctx context.Context, request UploadRequest, streaming bool) (*RawResponse, error) {
	if request.FileField == "" || request.FileName == "" {
		return nil, errors.New("upload requires file field and filename")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	payload, contentType, err := buildUploadPayload(request)
	if err != nil {
		return nil, err
	}
	var result *RawResponse
	err = c.withRetry(ctx, func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		req, err := c.buildRequest(ctx, http.MethodPost, request.Path, request.Query, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		mergeHeaders(req.Header, request.Headers)
		req.Header.Set("Content-Type", contentType)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		if streaming {
			opened, err := c.validateStreamResponse(resp)
			if err != nil {
				return err
			}
			result = &RawResponse{Body: opened.Body, Meta: opened.Meta}
			return nil
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read upload response: %w", err)
		}
		meta := extractResponseMeta(resp, body)
		if err := protocol.CheckResponseWithTrace(resp.StatusCode, body, protocol.TraceMeta{RequestID: meta.RequestID, TraceID: meta.TraceID}); err != nil {
			return err
		}
		result = &RawResponse{Body: io.NopCloser(bytes.NewReader(body)), Meta: meta}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
