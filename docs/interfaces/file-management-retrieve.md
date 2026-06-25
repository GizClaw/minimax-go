# File Retrieve

- Official docs: https://platform.minimaxi.com/docs/api-reference/file-management-retrieve.md
- Endpoint: `GET /v1/files/retrieve`
- SDK status: `Implemented`
- Local code: `File.Retrieve` in `file.go`; tests in `file_test.go`.

## Purpose

Retrieve file metadata, including generated file download information when
available.

## Current SDK shape

`File.Retrieve` requires a non-empty `file_id` and returns normalized file
metadata, including `download_url` when present, while preserving raw metadata.
