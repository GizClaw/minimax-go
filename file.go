package minimax

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/GizClaw/minimax-go/internal/transport"
)

const (
	defaultFileUploadPath     = "/v1/files/upload"
	defaultFileFieldName      = "file"
	defaultFileContentType    = "application/octet-stream"
	defaultFileMaxUploadBytes = 20 << 20 // 20 MiB
	fileUploadPurposeField    = "purpose"
)

type FileService struct {
	transport      *transport.Client
	uploadEndpoint string
	maxUploadBytes int
}

type FileUploadRequest struct {
	Purpose     string `json:"purpose,omitempty"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type,omitempty"`
	Data        []byte `json:"-"`
}

type FileMeta struct {
	FileName    string `json:"file_name,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Size        int64  `json:"size,omitempty"`
}

type FileUploadResponse struct {
	FileID   string   `json:"file_id,omitempty"`
	FileURL  string   `json:"file_url,omitempty"`
	Uploaded bool     `json:"uploaded"`
	Meta     FileMeta `json:"meta,omitempty"`
}

type flexibleString string

type fileUploadRawResponse struct {
	Uploaded    bool                  `json:"uploaded,omitempty"`
	FileID      flexibleString        `json:"file_id,omitempty"`
	ID          flexibleString        `json:"id,omitempty"`
	FileURL     string                `json:"file_url,omitempty"`
	URL         string                `json:"url,omitempty"`
	FileName    string                `json:"file_name,omitempty"`
	Name        string                `json:"name,omitempty"`
	ContentType string                `json:"content_type,omitempty"`
	MIMEType    string                `json:"mime_type,omitempty"`
	Size        *int64                `json:"size,omitempty"`
	Bytes       *int64                `json:"bytes,omitempty"`
	Data        *fileUploadRawPayload `json:"data,omitempty"`
	File        *fileUploadRawPayload `json:"file,omitempty"`
	Result      *fileUploadRawPayload `json:"result,omitempty"`
}

type fileUploadRawPayload struct {
	Uploaded    *bool          `json:"uploaded,omitempty"`
	FileID      flexibleString `json:"file_id,omitempty"`
	ID          flexibleString `json:"id,omitempty"`
	FileURL     string         `json:"file_url,omitempty"`
	URL         string         `json:"url,omitempty"`
	FileName    string         `json:"file_name,omitempty"`
	Name        string         `json:"name,omitempty"`
	ContentType string         `json:"content_type,omitempty"`
	MIMEType    string         `json:"mime_type,omitempty"`
	Size        *int64         `json:"size,omitempty"`
	Bytes       *int64         `json:"bytes,omitempty"`
}

// Upload uploads file bytes through multipart/form-data and returns normalized metadata.
func (s *FileService) Upload(ctx context.Context, request FileUploadRequest) (*FileUploadResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("file service is not initialized")
	}

	request.Purpose = strings.TrimSpace(request.Purpose)
	request.FileName = strings.TrimSpace(request.FileName)
	request.ContentType = strings.TrimSpace(request.ContentType)

	if request.FileName == "" {
		return nil, errors.New("file upload file name is empty")
	}

	if len(request.Data) == 0 {
		return nil, errors.New("file upload data is empty")
	}

	maxUploadBytes := s.resolveMaxUploadBytes()
	if len(request.Data) > maxUploadBytes {
		return nil, fmt.Errorf("file upload data exceeds max size: got=%d max=%d", len(request.Data), maxUploadBytes)
	}

	contentType, err := resolveFileContentType(request.FileName, request.ContentType)
	if err != nil {
		return nil, err
	}

	fields := make(map[string]string, 1)
	if request.Purpose != "" {
		fields[fileUploadPurposeField] = request.Purpose
	}
	if len(fields) == 0 {
		fields = nil
	}

	var raw fileUploadRawResponse
	if err := s.transport.Upload(ctx, transport.UploadRequest{
		Method:          http.MethodPost,
		Path:            s.resolveUploadPath(),
		Fields:          fields,
		FileField:       defaultFileFieldName,
		FileName:        request.FileName,
		FileContentType: contentType,
		FileData:        request.Data,
	}, &raw); err != nil {
		return nil, err
	}

	return mapFileUploadResponse(raw, request, contentType), nil
}

func (s *FileService) resolveUploadPath() string {
	uploadPath := strings.TrimSpace(s.uploadEndpoint)
	if uploadPath != "" {
		return uploadPath
	}

	return defaultFileUploadPath
}

func (s *FileService) resolveMaxUploadBytes() int {
	if s.maxUploadBytes > 0 {
		return s.maxUploadBytes
	}

	return defaultFileMaxUploadBytes
}

func resolveFileContentType(fileName, contentType string) (string, error) {
	if contentType != "" {
		mediaType, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			return "", fmt.Errorf("file upload content type is invalid: %w", err)
		}

		if !strings.Contains(mediaType, "/") {
			return "", errors.New("file upload content type is invalid: missing type/subtype separator")
		}

		normalized := mime.FormatMediaType(mediaType, params)
		if normalized == "" {
			return "", errors.New("file upload content type is invalid: unable to normalize media type")
		}

		return normalized, nil
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	if ext != "" {
		if inferred := strings.TrimSpace(mime.TypeByExtension(ext)); inferred != "" {
			return inferred, nil
		}
	}

	return defaultFileContentType, nil
}

func mapFileUploadResponse(raw fileUploadRawResponse, request FileUploadRequest, contentType string) *FileUploadResponse {
	payload := firstNonNilUploadPayload(raw.Data, raw.File, raw.Result)

	response := &FileUploadResponse{
		FileID:   firstNonEmptyValue(raw.FileID.String(), raw.ID.String()),
		FileURL:  firstNonEmptyValue(raw.FileURL, raw.URL),
		Uploaded: raw.Uploaded,
		Meta: FileMeta{
			FileName:    request.FileName,
			ContentType: contentType,
			Size:        int64(len(request.Data)),
		},
	}

	response.Meta.FileName = firstNonEmptyValue(raw.FileName, raw.Name, response.Meta.FileName)
	response.Meta.ContentType = firstNonEmptyValue(raw.ContentType, raw.MIMEType, response.Meta.ContentType)
	if size, ok := firstNonNilInt64(raw.Size, raw.Bytes); ok {
		response.Meta.Size = size
	}

	if payload != nil {
		response.FileID = firstNonEmptyValue(response.FileID, payload.FileID.String(), payload.ID.String())
		response.FileURL = firstNonEmptyValue(response.FileURL, payload.FileURL, payload.URL)
		response.Meta.FileName = firstNonEmptyValue(payload.FileName, payload.Name, response.Meta.FileName)
		response.Meta.ContentType = firstNonEmptyValue(payload.ContentType, payload.MIMEType, response.Meta.ContentType)
		if size, ok := firstNonNilInt64(payload.Size, payload.Bytes); ok {
			response.Meta.Size = size
		}
		if payload.Uploaded != nil {
			response.Uploaded = *payload.Uploaded
		}
	}

	if !response.Uploaded && (response.FileID != "" || response.FileURL != "") {
		response.Uploaded = true
	}

	return response
}

func firstNonNilUploadPayload(payloads ...*fileUploadRawPayload) *fileUploadRawPayload {
	for _, payload := range payloads {
		if payload != nil {
			return payload
		}
	}

	return nil
}

func (s flexibleString) String() string {
	return string(s)
}

func (s *flexibleString) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*s = ""
		return nil
	}

	var str string
	if err := json.Unmarshal(trimmed, &str); err == nil {
		*s = flexibleString(strings.TrimSpace(str))
		return nil
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()

	var number json.Number
	if err := decoder.Decode(&number); err == nil {
		*s = flexibleString(number.String())
		return nil
	}

	return errors.New("invalid string-like field value")
}

func firstNonEmptyValue(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func firstNonNilInt64(values ...*int64) (int64, bool) {
	for _, value := range values {
		if value != nil {
			return *value, true
		}
	}

	return 0, false
}
