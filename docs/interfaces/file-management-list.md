# File List

- Official docs: https://platform.minimaxi.com/docs/api-reference/file-management-list.md
- Endpoint: `GET /v1/files/list`
- SDK status: `Implemented`
- Local code: `File.List` in `file.go`; tests in `file_test.go`.

## Purpose

List files stored in the MiniMax file system by category and pagination
parameters.

## Current SDK shape

`FileListRequest` requires `purpose`. `FileListResponse` returns normalized
`FileInfo` values while preserving unknown file metadata in `Raw`.
