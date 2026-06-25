package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"github.com/GizClaw/minimax-go/internal/protocol"
)

const (
	defaultMaxAttempts = 3
	defaultBaseDelay   = 100 * time.Millisecond
	defaultMaxDelay    = 2 * time.Second
)

type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	ShouldRetry func(error) bool
	Sleep       func(context.Context, time.Duration) error
}

type Config struct {
	BaseURL        string
	APIKey         string
	HTTPClient     *http.Client
	DefaultHeaders http.Header
	Retry          RetryConfig
}

type Client struct {
	baseURL        string
	apiKey         string
	httpClient     *http.Client
	defaultHeaders http.Header
	retry          RetryConfig
}

type ResponseMeta struct {
	RequestID  string
	TraceID    string
	HTTPStatus int
	Header     http.Header
}

type JSONRequest struct {
	Method  string
	Path    string
	Query   url.Values
	Headers http.Header
	Body    any
}

type StreamRequest struct {
	Method  string
	Path    string
	Query   url.Values
	Headers http.Header
	Body    any
}

type RawRequest struct {
	Method  string
	Path    string
	Query   url.Values
	Headers http.Header
}

type UploadRequest struct {
	Method          string
	Path            string
	Query           url.Values
	Headers         http.Header
	Fields          map[string]string
	FileField       string
	FileName        string
	FileContentType string
	FileData        []byte
}

type WebSocketRequest struct {
	Path    string
	Query   url.Values
	Headers http.Header
}

type WebSocketDialConfig struct {
	URL    string
	Header http.Header
}

func New(config Config) (*Client, error) {
	retry := withRetryDefaults(config.Retry)

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	return &Client{
		baseURL:        strings.TrimSpace(config.BaseURL),
		apiKey:         strings.TrimSpace(config.APIKey),
		httpClient:     httpClient,
		defaultHeaders: config.DefaultHeaders.Clone(),
		retry:          retry,
	}, nil
}

// DoJSON sends a JSON request and unmarshals the response into out.
func (c *Client) DoJSON(ctx context.Context, request JSONRequest, out any) error {
	_, err := c.DoJSONWithMeta(ctx, request, out)
	return err
}

// DoJSONWithMeta sends a JSON request, unmarshals the response, and returns response metadata.
func (c *Client) DoJSONWithMeta(ctx context.Context, request JSONRequest, out any) (ResponseMeta, error) {
	method := request.Method
	if method == "" {
		method = http.MethodPost
	}

	payload, err := marshalRequestBody(request.Body, "marshal request body")
	if err != nil {
		return ResponseMeta{}, err
	}

	var meta ResponseMeta
	err = c.withRetry(ctx, func() error {
		req, reqErr := c.buildRequest(ctx, method, request.Path, request.Query, bytes.NewReader(payload))
		if reqErr != nil {
			return reqErr
		}

		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		mergeHeaders(req.Header, request.Headers)

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			return doErr
		}
		defer resp.Body.Close()

		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("read response body: %w", readErr)
		}

		meta = extractResponseMeta(resp, body)
		if checkErr := protocol.CheckResponseWithTrace(resp.StatusCode, body, protocol.TraceMeta{
			RequestID: meta.RequestID,
			TraceID:   meta.TraceID,
		}); checkErr != nil {
			return checkErr
		}

		return decodeResponseBody(body, out, "decode response body")
	})
	if err != nil {
		return ResponseMeta{}, err
	}

	return meta, nil
}

// OpenStream opens a streaming connection; caller must close the returned body.
func (c *Client) OpenStream(ctx context.Context, request StreamRequest) (io.ReadCloser, error) {
	streamResponse, err := c.OpenStreamWithMeta(ctx, request)
	if err != nil {
		return nil, err
	}
	return streamResponse.Body, nil
}

type StreamResponse struct {
	Body io.ReadCloser
	Meta ResponseMeta
}

type RawResponse struct {
	Body io.ReadCloser
	Meta ResponseMeta
}

// OpenStreamWithMeta opens a streaming connection and returns response metadata.
func (c *Client) OpenStreamWithMeta(ctx context.Context, request StreamRequest) (*StreamResponse, error) {
	method := request.Method
	if method == "" {
		method = http.MethodGet
	}

	payload, err := marshalRequestBody(request.Body, "marshal stream body")
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 1; attempt <= c.retry.MaxAttempts; attempt++ {
		streamResponse, openErr := c.openStreamAttempt(ctx, method, payload, request)
		if openErr == nil {
			return streamResponse, nil
		}

		lastErr = openErr
		if !c.shouldRetry(openErr) || attempt == c.retry.MaxAttempts {
			return nil, openErr
		}

		if sleepErr := c.retry.Sleep(ctx, c.retryDelay(attempt)); sleepErr != nil {
			return nil, sleepErr
		}
	}

	if lastErr == nil {
		lastErr = errors.New("open stream failed")
	}

	return nil, lastErr
}

// OpenRawWithMeta opens a raw response body and returns response metadata.
func (c *Client) OpenRawWithMeta(ctx context.Context, request RawRequest) (*RawResponse, error) {
	method := request.Method
	if method == "" {
		method = http.MethodGet
	}

	var lastErr error
	for attempt := 1; attempt <= c.retry.MaxAttempts; attempt++ {
		rawResponse, openErr := c.openRawAttempt(ctx, method, request)
		if openErr == nil {
			return rawResponse, nil
		}

		lastErr = openErr
		if !c.shouldRetry(openErr) || attempt == c.retry.MaxAttempts {
			return nil, openErr
		}

		if sleepErr := c.retry.Sleep(ctx, c.retryDelay(attempt)); sleepErr != nil {
			return nil, sleepErr
		}
	}

	if lastErr == nil {
		lastErr = errors.New("open raw response failed")
	}

	return nil, lastErr
}

// OpenRawURLWithMeta opens an absolute raw URL without adding API authorization.
func (c *Client) OpenRawURLWithMeta(ctx context.Context, rawURL string) (*RawResponse, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, errors.New("raw URL is empty")
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil, errors.New("raw URL must be absolute")
	}

	var lastErr error
	for attempt := 1; attempt <= c.retry.MaxAttempts; attempt++ {
		rawResponse, openErr := c.openRawURLAttempt(ctx, rawURL)
		if openErr == nil {
			return rawResponse, nil
		}

		lastErr = openErr
		if !c.shouldRetry(openErr) || attempt == c.retry.MaxAttempts {
			return nil, openErr
		}

		if sleepErr := c.retry.Sleep(ctx, c.retryDelay(attempt)); sleepErr != nil {
			return nil, sleepErr
		}
	}

	if lastErr == nil {
		lastErr = errors.New("open raw URL response failed")
	}

	return nil, lastErr
}

// Upload sends a multipart/form-data request.
func (c *Client) Upload(ctx context.Context, request UploadRequest, out any) error {
	_, err := c.UploadWithMeta(ctx, request, out)
	return err
}

// UploadWithMeta sends a multipart/form-data request and returns response metadata.
func (c *Client) UploadWithMeta(ctx context.Context, request UploadRequest, out any) (ResponseMeta, error) {
	method := request.Method
	if method == "" {
		method = http.MethodPost
	}

	if request.FileField == "" {
		return ResponseMeta{}, errors.New("upload request requires FileField")
	}

	if request.FileName == "" {
		return ResponseMeta{}, errors.New("upload request requires FileName")
	}

	var meta ResponseMeta
	err := c.withRetry(ctx, func() error {
		payload, contentType, err := buildUploadPayload(request)
		if err != nil {
			return err
		}

		req, reqErr := c.buildRequest(ctx, method, request.Path, request.Query, bytes.NewReader(payload))
		if reqErr != nil {
			return reqErr
		}

		req.Header.Set("Accept", "application/json")
		mergeHeaders(req.Header, request.Headers)
		req.Header.Set("Content-Type", contentType)

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			return doErr
		}
		defer resp.Body.Close()

		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("read upload response: %w", readErr)
		}

		meta = extractResponseMeta(resp, body)
		if checkErr := protocol.CheckResponseWithTrace(resp.StatusCode, body, protocol.TraceMeta{
			RequestID: meta.RequestID,
			TraceID:   meta.TraceID,
		}); checkErr != nil {
			return checkErr
		}

		return decodeResponseBody(body, out, "decode upload response")
	})
	if err != nil {
		return ResponseMeta{}, err
	}

	return meta, nil
}

// BuildWebSocketDialConfig resolves a WebSocket endpoint and headers using the
// same base URL, default headers, and bearer authorization as HTTP requests.
func (c *Client) BuildWebSocketDialConfig(ctx context.Context, request WebSocketRequest) (*WebSocketDialConfig, error) {
	req, err := c.buildRequest(ctx, http.MethodGet, request.Path, request.Query, nil)
	if err != nil {
		return nil, err
	}
	mergeHeaders(req.Header, request.Headers)

	switch req.URL.Scheme {
	case "https":
		req.URL.Scheme = "wss"
	case "http":
		req.URL.Scheme = "ws"
	case "ws", "wss":
	default:
		return nil, fmt.Errorf("unsupported websocket URL scheme %q", req.URL.Scheme)
	}

	return &WebSocketDialConfig{
		URL:    req.URL.String(),
		Header: req.Header.Clone(),
	}, nil
}

func (c *Client) openRawAttempt(ctx context.Context, method string, request RawRequest) (*RawResponse, error) {
	req, reqErr := c.buildRequest(ctx, method, request.Path, request.Query, nil)
	if reqErr != nil {
		return nil, reqErr
	}

	req.Header.Set("Accept", "*/*")
	mergeHeaders(req.Header, request.Headers)

	resp, doErr := c.httpClient.Do(req)
	if doErr != nil {
		return nil, doErr
	}

	meta := extractResponseMeta(resp, nil)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, readResponseError(resp, "read raw error response")
	}

	if isJSONContentType(resp.Header.Get("Content-Type")) {
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read raw response body: %w", readErr)
		}

		meta = extractResponseMeta(resp, body)
		if checkErr := protocol.CheckResponseWithTrace(resp.StatusCode, body, protocol.TraceMeta{
			RequestID: meta.RequestID,
			TraceID:   meta.TraceID,
		}); checkErr != nil {
			return nil, checkErr
		}

		return &RawResponse{
			Body: io.NopCloser(bytes.NewReader(body)),
			Meta: meta,
		}, nil
	}

	return &RawResponse{
		Body: resp.Body,
		Meta: meta,
	}, nil
}

func (c *Client) openRawURLAttempt(ctx context.Context, rawURL string) (*RawResponse, error) {
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if reqErr != nil {
		return nil, fmt.Errorf("new raw URL request: %w", reqErr)
	}
	req.Header.Set("Accept", "*/*")

	resp, doErr := c.httpClient.Do(req)
	if doErr != nil {
		return nil, doErr
	}

	meta := extractResponseMeta(resp, nil)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, readResponseError(resp, "read raw URL error response")
	}

	return &RawResponse{
		Body: resp.Body,
		Meta: meta,
	}, nil
}

func (c *Client) openStreamAttempt(ctx context.Context, method string, payload []byte, request StreamRequest) (*StreamResponse, error) {
	req, reqErr := c.buildRequest(ctx, method, request.Path, request.Query, bytes.NewReader(payload))
	if reqErr != nil {
		return nil, reqErr
	}

	req.Header.Set("Accept", "text/event-stream")
	if len(payload) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	mergeHeaders(req.Header, request.Headers)

	resp, doErr := c.httpClient.Do(req)
	if doErr != nil {
		return nil, doErr
	}

	return c.validateStreamResponse(resp)
}

func (c *Client) validateStreamResponse(resp *http.Response) (*StreamResponse, error) {
	meta := extractResponseMeta(resp, nil)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, readResponseError(resp, "read stream error response")
	}

	contentType := resp.Header.Get("Content-Type")
	if isEventStreamContentType(contentType) {
		return &StreamResponse{
			Body: resp.Body,
			Meta: meta,
		}, nil
	}

	if err := readResponseError(resp, "read non-stream response body"); err != nil {
		return nil, err
	}

	return nil, fmt.Errorf("unexpected stream content type: %q", contentType)
}

func readResponseError(resp *http.Response, readErrPrefix string) error {
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return fmt.Errorf("%s: %w", readErrPrefix, err)
	}

	meta := extractResponseMeta(resp, body)
	return protocol.CheckResponseWithTrace(resp.StatusCode, body, protocol.TraceMeta{
		RequestID: meta.RequestID,
		TraceID:   meta.TraceID,
	})
}

func marshalRequestBody(body any, errorPrefix string) ([]byte, error) {
	if body == nil {
		return nil, nil
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errorPrefix, err)
	}

	return payload, nil
}

func decodeResponseBody(body []byte, out any, errorPrefix string) error {
	if out == nil || len(body) == 0 {
		return nil
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%s: %w", errorPrefix, err)
	}

	return nil
}

func extractResponseMeta(resp *http.Response, body []byte) ResponseMeta {
	if resp == nil {
		return ResponseMeta{}
	}

	bodyTrace := protocol.ExtractTraceMeta(body)
	headerTrace := extractHeaderTraceMeta(resp.Header)

	return ResponseMeta{
		RequestID:  firstNonEmpty(headerTrace.RequestID, bodyTrace.RequestID),
		TraceID:    firstNonEmpty(headerTrace.TraceID, bodyTrace.TraceID),
		HTTPStatus: resp.StatusCode,
		Header:     resp.Header.Clone(),
	}
}

func extractHeaderTraceMeta(header http.Header) protocol.TraceMeta {
	if header == nil {
		return protocol.TraceMeta{}
	}

	return protocol.TraceMeta{
		RequestID: firstNonEmpty(
			header.Get("X-Request-ID"),
			header.Get("X-Request-Id"),
			header.Get("X-Minimax-Request-ID"),
			header.Get("MiniMax-Request-ID"),
			header.Get("Request-ID"),
		),
		TraceID: firstNonEmpty(
			header.Get("X-Trace-ID"),
			header.Get("X-Trace-Id"),
			header.Get("X-Minimax-Trace-ID"),
			header.Get("MiniMax-Trace-ID"),
			header.Get("Trace-ID"),
		),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func buildUploadPayload(request UploadRequest) ([]byte, string, error) {
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)

	for key, value := range request.Fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, "", fmt.Errorf("write field %s: %w", key, err)
		}
	}

	header := textproto.MIMEHeader{}
	contentDisposition := fmt.Sprintf(`form-data; name=%q; filename=%q`, request.FileField, request.FileName)
	header.Set("Content-Disposition", contentDisposition)
	if request.FileContentType != "" {
		header.Set("Content-Type", request.FileContentType)
	}

	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, "", fmt.Errorf("create file part: %w", err)
	}

	if _, err := part.Write(request.FileData); err != nil {
		return nil, "", fmt.Errorf("write file data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("close multipart writer: %w", err)
	}

	return payload.Bytes(), writer.FormDataContentType(), nil
}

func (c *Client) withRetry(ctx context.Context, op func() error) error {
	var lastErr error

	for attempt := 1; attempt <= c.retry.MaxAttempts; attempt++ {
		err := op()
		if err == nil {
			return nil
		}

		lastErr = err
		if !c.shouldRetry(err) || attempt == c.retry.MaxAttempts {
			return err
		}

		if sleepErr := c.retry.Sleep(ctx, c.retryDelay(attempt)); sleepErr != nil {
			return sleepErr
		}
	}

	if lastErr == nil {
		return errors.New("request failed")
	}

	return lastErr
}

func (c *Client) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	if c.retry.ShouldRetry != nil {
		return c.retry.ShouldRetry(err)
	}

	return protocol.IsRetryable(err)
}

func (c *Client) retryDelay(attempt int) time.Duration {
	delay := c.retry.BaseDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= c.retry.MaxDelay {
			return c.retry.MaxDelay
		}
	}

	if delay > c.retry.MaxDelay {
		return c.retry.MaxDelay
	}

	return delay
}

func withRetryDefaults(retry RetryConfig) RetryConfig {
	if retry.MaxAttempts <= 0 {
		retry.MaxAttempts = defaultMaxAttempts
	}

	if retry.BaseDelay <= 0 {
		retry.BaseDelay = defaultBaseDelay
	}

	if retry.MaxDelay <= 0 {
		retry.MaxDelay = defaultMaxDelay
	}

	if retry.Sleep == nil {
		retry.Sleep = sleepWithContext
	}

	return retry
}

func (c *Client) buildRequest(ctx context.Context, method, path string, query url.Values, body io.Reader) (*http.Request, error) {
	resolvedURL, err := c.resolveURL(path)
	if err != nil {
		return nil, err
	}

	parsedURL, err := url.Parse(resolvedURL)
	if err != nil {
		return nil, fmt.Errorf("parse request url: %w", err)
	}

	if query != nil {
		q := parsedURL.Query()
		for key, values := range query {
			for _, value := range values {
				q.Add(key, value)
			}
		}
		parsedURL.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, parsedURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	mergeHeaders(req.Header, c.defaultHeaders)
	if c.apiKey != "" && req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	return req, nil
}

func (c *Client) resolveURL(path string) (string, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return "", errors.New("request path is empty")
	}

	if strings.HasPrefix(trimmedPath, "http://") || strings.HasPrefix(trimmedPath, "https://") {
		return trimmedPath, nil
	}

	if c.baseURL == "" {
		return "", errors.New("baseURL is empty for relative path request")
	}

	return strings.TrimRight(c.baseURL, "/") + "/" + strings.TrimLeft(trimmedPath, "/"), nil
}

func mergeHeaders(dst, src http.Header) {
	if dst == nil || src == nil {
		return
	}

	for key, values := range src {
		dst.Del(key)
		for idx, value := range values {
			if idx == 0 {
				dst.Set(key, value)
				continue
			}
			dst.Add(key, value)
		}
	}
}

func isEventStreamContentType(contentType string) bool {
	trimmed := strings.TrimSpace(contentType)
	if trimmed == "" {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(trimmed)
	if err == nil {
		return strings.EqualFold(mediaType, "text/event-stream")
	}

	return strings.HasPrefix(strings.ToLower(trimmed), "text/event-stream")
}

func isJSONContentType(contentType string) bool {
	trimmed := strings.TrimSpace(contentType)
	if trimmed == "" {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(trimmed)
	if err == nil {
		return strings.EqualFold(mediaType, "application/json")
	}

	return strings.HasPrefix(strings.ToLower(trimmed), "application/json")
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
