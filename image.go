package minimax

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/GizClaw/minimax-go/internal/transport"
)

const defaultImageGenerationPath = "/v1/image_generation"

// ImageService provides MiniMax image generation APIs.
type ImageService struct {
	transport *transport.Client
	endpoint  string
}

// ImageTextToImageRequest contains parameters for MiniMax text-to-image generation.
type ImageTextToImageRequest struct {
	Model           string      `json:"model"`
	Prompt          string      `json:"prompt"`
	Style           *ImageStyle `json:"style,omitempty"`
	AspectRatio     string      `json:"aspect_ratio,omitempty"`
	Width           *int        `json:"width,omitempty"`
	Height          *int        `json:"height,omitempty"`
	ResponseFormat  string      `json:"response_format,omitempty"`
	Seed            *int64      `json:"seed,omitempty"`
	N               *int        `json:"n,omitempty"`
	PromptOptimizer *bool       `json:"prompt_optimizer,omitempty"`
	AIGCWatermark   *bool       `json:"aigc_watermark,omitempty"`
}

// ImageStyle configures style controls for models that support them.
type ImageStyle struct {
	StyleType   string   `json:"style_type"`
	StyleWeight *float64 `json:"style_weight,omitempty"`
}

// ImageGenerationResponse is a normalized text-to-image generation response.
type ImageGenerationResponse struct {
	ResponseMeta ResponseMeta               `json:"response_meta,omitzero"`
	ID           string                     `json:"id,omitempty"`
	ImageURLs    []string                   `json:"image_urls,omitempty"`
	ImageBase64  []string                   `json:"image_base64,omitempty"`
	Metadata     ImageGenerationMetadata    `json:"metadata"`
	Raw          map[string]json.RawMessage `json:"-"`
}

// ImageGenerationMetadata reports image generation success and safety-filter counts.
type ImageGenerationMetadata struct {
	SuccessCount *int `json:"success_count,omitempty"`
	FailedCount  *int `json:"failed_count,omitempty"`
}

type imageGenerationRawResponse struct {
	ID       string                      `json:"id,omitempty"`
	Data     *imageGenerationRawData     `json:"data,omitempty"`
	Metadata *imageGenerationRawMetadata `json:"metadata,omitempty"`
	Raw      map[string]json.RawMessage  `json:"-"`
}

type imageGenerationRawData struct {
	ImageURLs   []string `json:"image_urls,omitempty"`
	ImageBase64 []string `json:"image_base64,omitempty"`
}

type imageGenerationRawMetadata struct {
	SuccessCount optionalInt `json:"success_count,omitempty"`
	FailedCount  optionalInt `json:"failed_count,omitempty"`
}

type optionalInt struct {
	value int
	set   bool
}

// GenerateTextToImage generates images from a text prompt.
func (s *ImageService) GenerateTextToImage(ctx context.Context, request ImageTextToImageRequest) (*ImageGenerationResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("image service is not initialized")
	}

	normalizeImageTextToImageRequest(&request)
	if err := validateImageTextToImageRequest(request); err != nil {
		return nil, err
	}

	var raw imageGenerationRawResponse
	meta, err := s.transport.DoJSONWithMeta(ctx, transport.JSONRequest{
		Method: http.MethodPost,
		Path:   s.resolvePath(),
		Body:   request,
	}, &raw)
	if err != nil {
		return nil, err
	}

	response := mapImageGenerationResponse(raw)
	response.ResponseMeta = responseMetaFromTransport(meta)
	return response, nil
}

func (s *ImageService) resolvePath() string {
	path := strings.TrimSpace(s.endpoint)
	if path != "" {
		return path
	}

	return defaultImageGenerationPath
}

func normalizeImageTextToImageRequest(request *ImageTextToImageRequest) {
	request.Model = strings.TrimSpace(request.Model)
	request.Prompt = strings.TrimSpace(request.Prompt)
	request.AspectRatio = strings.TrimSpace(request.AspectRatio)
	request.ResponseFormat = strings.TrimSpace(request.ResponseFormat)

	if request.Style != nil {
		style := *request.Style
		style.StyleType = strings.TrimSpace(style.StyleType)
		request.Style = &style
	}
}

func validateImageTextToImageRequest(request ImageTextToImageRequest) error {
	if request.Model == "" {
		return errors.New("image text-to-image request model is empty")
	}
	if request.Prompt == "" {
		return errors.New("image text-to-image request prompt is empty")
	}
	if request.Style != nil && request.Style.StyleType == "" {
		return errors.New("image text-to-image request style_type is empty")
	}
	if request.Style != nil && request.Style.StyleWeight != nil && (*request.Style.StyleWeight <= 0 || *request.Style.StyleWeight > 1) {
		return fmt.Errorf("image text-to-image request style_weight must be greater than 0 and no more than 1: %g", *request.Style.StyleWeight)
	}
	if (request.Width == nil) != (request.Height == nil) {
		return errors.New("image text-to-image request width and height must be provided together")
	}
	if request.Width != nil {
		if err := validateImageDimension("width", *request.Width); err != nil {
			return err
		}
		if err := validateImageDimension("height", *request.Height); err != nil {
			return err
		}
	}
	if request.N != nil && (*request.N < 1 || *request.N > 9) {
		return fmt.Errorf("image text-to-image request n must be between 1 and 9: %d", *request.N)
	}
	if request.ResponseFormat != "" && request.ResponseFormat != "url" && request.ResponseFormat != "base64" {
		return fmt.Errorf("image text-to-image request response_format must be url or base64: %s", request.ResponseFormat)
	}

	return nil
}

func validateImageDimension(name string, value int) error {
	if value < 512 || value > 2048 {
		return fmt.Errorf("image text-to-image request %s must be between 512 and 2048: %d", name, value)
	}
	if value%8 != 0 {
		return fmt.Errorf("image text-to-image request %s must be a multiple of 8: %d", name, value)
	}

	return nil
}

func mapImageGenerationResponse(raw imageGenerationRawResponse) *ImageGenerationResponse {
	response := &ImageGenerationResponse{
		ID:  strings.TrimSpace(raw.ID),
		Raw: cloneRawMessages(raw.Raw),
	}
	if raw.Data != nil {
		response.ImageURLs = append([]string(nil), raw.Data.ImageURLs...)
		response.ImageBase64 = append([]string(nil), raw.Data.ImageBase64...)
	}
	if raw.Metadata != nil {
		response.Metadata = ImageGenerationMetadata{
			SuccessCount: raw.Metadata.SuccessCount.IntPtr(),
			FailedCount:  raw.Metadata.FailedCount.IntPtr(),
		}
	}

	return response
}

func (r *imageGenerationRawResponse) UnmarshalJSON(data []byte) error {
	type alias imageGenerationRawResponse
	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	delete(raw, "id")
	delete(raw, "data")
	delete(raw, "metadata")
	delete(raw, "base_resp")
	delete(raw, "status_code")
	delete(raw, "status_msg")

	*r = imageGenerationRawResponse(parsed)
	if len(raw) > 0 {
		r.Raw = raw
	} else {
		r.Raw = nil
	}

	return nil
}

func (v *optionalInt) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*v = optionalInt{}
		return nil
	}

	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err == nil {
		parsed, parseErr := strconv.Atoi(number.String())
		if parseErr != nil {
			return fmt.Errorf("parse integer metadata value: %w", parseErr)
		}
		v.value = parsed
		v.set = true
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("parse integer metadata value: %w", err)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		*v = optionalInt{}
		return nil
	}

	parsed, err := strconv.Atoi(text)
	if err != nil {
		return fmt.Errorf("parse integer metadata value: %w", err)
	}
	v.value = parsed
	v.set = true
	return nil
}

func (v optionalInt) IntPtr() *int {
	if !v.set {
		return nil
	}

	copied := v.value
	return &copied
}
