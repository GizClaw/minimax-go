package minimax

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/GizClaw/minimax-go/internal/protocol"
	"github.com/GizClaw/minimax-go/internal/transport"
)

const (
	defaultFileUploadPath     = "/v1/files/upload"
	defaultFileListPath       = "/v1/files/list"
	defaultFileRetrievePath   = "/v1/files/retrieve"
	defaultFileDownloadPath   = "/v1/files/retrieve_content"
	defaultFileDeletePath     = "/v1/files/delete"
	defaultFileFieldName      = "file"
	defaultFileContentType    = "application/octet-stream"
	defaultFileMaxUploadBytes = 20 << 20 // 20 MiB
	fileUploadPurposeField    = "purpose"
)

type FileService struct {
	transport        *transport.Client
	uploadEndpoint   string
	listEndpoint     string
	retrieveEndpoint string
	downloadEndpoint string
	deleteEndpoint   string
	maxUploadBytes   int
}

type FileUploadRequest struct {
	Purpose     string `json:"purpose,omitempty"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type,omitempty"`
	Data        []byte `json:"-"`
}

type FileListRequest struct {
	Purpose string `json:"purpose"`
}

type FileDeleteRequest struct {
	FileID  string `json:"file_id"`
	Purpose string `json:"purpose"`
}

type fileDeleteWireRequest struct {
	FileID  numericStringID `json:"file_id"`
	Purpose string          `json:"purpose"`
}

type FileMeta struct {
	FileName    string `json:"file_name,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Size        int64  `json:"size,omitempty"`
}

type FileInfo struct {
	FileID      string                     `json:"file_id,omitempty"`
	Bytes       int64                      `json:"bytes,omitempty"`
	CreatedAt   int64                      `json:"created_at,omitempty"`
	FileName    string                     `json:"filename,omitempty"`
	Purpose     string                     `json:"purpose,omitempty"`
	DownloadURL string                     `json:"download_url,omitempty"`
	Raw         map[string]json.RawMessage `json:"-"`
}

type FileUploadResponse struct {
	ResponseMeta ResponseMeta `json:"response_meta,omitzero"`
	FileID       string       `json:"file_id,omitempty"`
	FileURL      string       `json:"file_url,omitempty"`
	Uploaded     bool         `json:"uploaded"`
	Meta         FileMeta     `json:"meta"`
}

type FileListResponse struct {
	ResponseMeta ResponseMeta `json:"response_meta,omitzero"`
	Files        []FileInfo   `json:"files"`
}

type FileRetrieveResponse struct {
	ResponseMeta ResponseMeta `json:"response_meta,omitzero"`
	File         FileInfo     `json:"file"`
}

type FileDownloadResponse struct {
	ResponseMeta  ResponseMeta  `json:"response_meta,omitzero"`
	Body          io.ReadCloser `json:"-"`
	ContentType   string        `json:"content_type,omitempty"`
	ContentLength int64         `json:"content_length,omitempty"`
}

type FileDeleteResponse struct {
	ResponseMeta ResponseMeta `json:"response_meta,omitzero"`
	FileID       string       `json:"file_id,omitempty"`
}

type flexibleString string

type fileListRawResponse struct {
	Files []fileRawObject `json:"files,omitempty"`
}

type fileRetrieveRawResponse struct {
	File fileRawObject `json:"file"`
}

type fileDeleteRawResponse struct {
	FileID flexibleString `json:"file_id,omitempty"`
	ID     flexibleString `json:"id,omitempty"`
}

type fileRawObject struct {
	FileID      flexibleString             `json:"file_id,omitempty"`
	ID          flexibleString             `json:"id,omitempty"`
	Bytes       *int64                     `json:"bytes,omitempty"`
	Size        *int64                     `json:"size,omitempty"`
	FileSize    *int64                     `json:"file_size,omitempty"`
	CreatedAt   *int64                     `json:"created_at,omitempty"`
	FileName    string                     `json:"filename,omitempty"`
	Name        string                     `json:"name,omitempty"`
	Purpose     string                     `json:"purpose,omitempty"`
	DownloadURL string                     `json:"download_url,omitempty"`
	FileURL     string                     `json:"file_url,omitempty"`
	URL         string                     `json:"url,omitempty"`
	Raw         map[string]json.RawMessage `json:"-"`
}

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
	meta, err := s.transport.UploadWithMeta(ctx, transport.UploadRequest{
		Method:          http.MethodPost,
		Path:            s.resolveUploadPath(),
		Fields:          fields,
		FileField:       defaultFileFieldName,
		FileName:        request.FileName,
		FileContentType: contentType,
		FileData:        request.Data,
	}, &raw)
	if err != nil {
		return nil, err
	}

	response := mapFileUploadResponse(raw, request, contentType)
	response.ResponseMeta = responseMetaFromTransport(meta)
	return response, nil
}

// List lists uploaded files for a MiniMax file purpose.
func (s *FileService) List(ctx context.Context, request FileListRequest) (*FileListResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("file service is not initialized")
	}

	request.Purpose = strings.TrimSpace(request.Purpose)
	if request.Purpose == "" {
		return nil, errors.New("file list purpose is empty")
	}

	query := url.Values{}
	query.Set(fileUploadPurposeField, request.Purpose)

	var raw fileListRawResponse
	meta, err := s.transport.DoJSONWithMeta(ctx, transport.JSONRequest{
		Method: http.MethodGet,
		Path:   s.resolveListPath(),
		Query:  query,
	}, &raw)
	if err != nil {
		return nil, err
	}

	files := make([]FileInfo, 0, len(raw.Files))
	for _, file := range raw.Files {
		files = append(files, mapFileInfo(file))
	}

	return &FileListResponse{
		ResponseMeta: responseMetaFromTransport(meta),
		Files:        files,
	}, nil
}

// Retrieve retrieves metadata for a MiniMax file.
func (s *FileService) Retrieve(ctx context.Context, fileID string) (*FileRetrieveResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("file service is not initialized")
	}

	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil, errors.New("file retrieve file_id is empty")
	}

	query := url.Values{}
	query.Set("file_id", fileID)

	var raw fileRetrieveRawResponse
	meta, err := s.transport.DoJSONWithMeta(ctx, transport.JSONRequest{
		Method: http.MethodGet,
		Path:   s.resolveRetrievePath(),
		Query:  query,
	}, &raw)
	if err != nil {
		return nil, err
	}

	return &FileRetrieveResponse{
		ResponseMeta: responseMetaFromTransport(meta),
		File:         mapFileInfo(raw.File),
	}, nil
}

// Download opens the raw content stream for a MiniMax file; callers must close Body.
func (s *FileService) Download(ctx context.Context, fileID string) (*FileDownloadResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("file service is not initialized")
	}

	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil, errors.New("file download file_id is empty")
	}

	query := url.Values{}
	query.Set("file_id", fileID)

	rawResponse, err := s.transport.OpenRawWithMeta(ctx, transport.RawRequest{
		Method: http.MethodGet,
		Path:   s.resolveDownloadPath(),
		Query:  query,
	})
	if err != nil {
		if shouldFallbackToRetrievedDownloadURL(err) {
			return s.downloadFromRetrievedURL(ctx, fileID)
		}
		return nil, err
	}

	return fileDownloadResponseFromRaw(rawResponse), nil
}

// Delete deletes a MiniMax file for a purpose.
func (s *FileService) Delete(ctx context.Context, request FileDeleteRequest) (*FileDeleteResponse, error) {
	if s == nil || s.transport == nil {
		return nil, errors.New("file service is not initialized")
	}

	request.FileID = strings.TrimSpace(request.FileID)
	request.Purpose = strings.TrimSpace(request.Purpose)
	if request.FileID == "" {
		return nil, errors.New("file delete file_id is empty")
	}
	if request.Purpose == "" {
		return nil, errors.New("file delete purpose is empty")
	}

	var raw fileDeleteRawResponse
	meta, err := s.transport.DoJSONWithMeta(ctx, transport.JSONRequest{
		Method: http.MethodPost,
		Path:   s.resolveDeletePath(),
		Body: fileDeleteWireRequest{
			FileID:  numericStringID(request.FileID),
			Purpose: request.Purpose,
		},
	}, &raw)
	if err != nil {
		return nil, err
	}

	return &FileDeleteResponse{
		ResponseMeta: responseMetaFromTransport(meta),
		FileID:       firstNonEmptyValue(raw.FileID.String(), raw.ID.String(), request.FileID),
	}, nil
}

func (s *FileService) resolveUploadPath() string {
	uploadPath := strings.TrimSpace(s.uploadEndpoint)
	if uploadPath != "" {
		return uploadPath
	}

	return defaultFileUploadPath
}

func (s *FileService) resolveListPath() string {
	listPath := strings.TrimSpace(s.listEndpoint)
	if listPath != "" {
		return listPath
	}

	return defaultFileListPath
}

func (s *FileService) resolveRetrievePath() string {
	retrievePath := strings.TrimSpace(s.retrieveEndpoint)
	if retrievePath != "" {
		return retrievePath
	}

	return defaultFileRetrievePath
}

func (s *FileService) resolveDownloadPath() string {
	downloadPath := strings.TrimSpace(s.downloadEndpoint)
	if downloadPath != "" {
		return downloadPath
	}

	return defaultFileDownloadPath
}

func (s *FileService) resolveDeletePath() string {
	deletePath := strings.TrimSpace(s.deleteEndpoint)
	if deletePath != "" {
		return deletePath
	}

	return defaultFileDeletePath
}

func (s *FileService) resolveMaxUploadBytes() int {
	if s.maxUploadBytes > 0 {
		return s.maxUploadBytes
	}

	return defaultFileMaxUploadBytes
}

func (s *FileService) downloadFromRetrievedURL(ctx context.Context, fileID string) (*FileDownloadResponse, error) {
	retrieved, err := s.Retrieve(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("file download retrieve fallback failed: %w", err)
	}

	downloadURL := strings.TrimSpace(retrieved.File.DownloadURL)
	if downloadURL == "" {
		return nil, errors.New("file download retrieve fallback missing download_url")
	}

	rawResponse, err := s.transport.OpenRawURLWithMeta(ctx, downloadURL)
	if err != nil {
		return nil, fmt.Errorf("file download_url failed: %w", err)
	}

	return fileDownloadResponseFromRaw(rawResponse), nil
}

func fileDownloadResponseFromRaw(rawResponse *transport.RawResponse) *FileDownloadResponse {
	if rawResponse == nil {
		return &FileDownloadResponse{}
	}

	return &FileDownloadResponse{
		ResponseMeta:  responseMetaFromTransport(rawResponse.Meta),
		Body:          rawResponse.Body,
		ContentType:   strings.TrimSpace(rawResponse.Meta.Header.Get("Content-Type")),
		ContentLength: contentLengthFromHeader(rawResponse.Meta.Header),
	}
}

func shouldFallbackToRetrievedDownloadURL(err error) bool {
	var apiErr *protocol.APIError
	if !errors.As(err, &apiErr) {
		return false
	}

	return apiErr.StatusCode == 2013 && strings.Contains(strings.ToLower(apiErr.StatusMsg), "file purpose")
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

func mapFileInfo(raw fileRawObject) FileInfo {
	info := FileInfo{
		FileID:      firstNonEmptyValue(raw.FileID.String(), raw.ID.String()),
		FileName:    firstNonEmptyValue(raw.FileName, raw.Name),
		Purpose:     strings.TrimSpace(raw.Purpose),
		DownloadURL: firstNonEmptyValue(raw.DownloadURL, raw.FileURL, raw.URL),
		Raw:         cloneRawMessages(raw.Raw),
	}

	if bytes, ok := firstNonNilInt64(raw.Bytes, raw.Size, raw.FileSize); ok {
		info.Bytes = bytes
	}
	if raw.CreatedAt != nil {
		info.CreatedAt = *raw.CreatedAt
	}

	return info
}

func contentLengthFromHeader(header http.Header) int64 {
	if header == nil {
		return 0
	}

	contentLength := strings.TrimSpace(header.Get("Content-Length"))
	if contentLength == "" {
		return 0
	}

	value, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil || value < 0 {
		return 0
	}

	return value
}

func firstNonNilUploadPayload(payloads ...*fileUploadRawPayload) *fileUploadRawPayload {
	for _, payload := range payloads {
		if payload != nil {
			return payload
		}
	}

	return nil
}

func (f *fileRawObject) UnmarshalJSON(data []byte) error {
	type alias fileRawObject
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	delete(raw, "file_id")
	delete(raw, "id")
	delete(raw, "bytes")
	delete(raw, "size")
	delete(raw, "file_size")
	delete(raw, "created_at")
	delete(raw, "filename")
	delete(raw, "name")
	delete(raw, "purpose")
	delete(raw, "download_url")
	delete(raw, "file_url")
	delete(raw, "url")

	*f = fileRawObject(decoded)
	f.Raw = raw
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
