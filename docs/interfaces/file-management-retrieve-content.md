# File Download

- Official docs: https://platform.minimaxi.com/docs/api-reference/file-management-retrieve-content.md
- Endpoint: `GET /v1/files/retrieve_content`
- SDK status: `Implemented`
- Local code: `File.Download` in `file.go`; raw body transport in `internal/transport`.

## Purpose

Download file bytes from MiniMax storage.

## Current SDK shape

`File.Download` requires a non-empty `file_id` and returns an `io.ReadCloser`
body with response metadata, content type, and content length. Callers must close
the body.
