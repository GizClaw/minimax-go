# File Delete

- Official docs: https://platform.minimaxi.com/docs/api-reference/file-management-delete.md
- Endpoint: `POST /v1/files/delete`
- SDK status: `Implemented`
- Local code: `File.Delete` in `file.go`; tests in `file_test.go`.

## Purpose

Delete a stored MiniMax file.

## Current SDK shape

`File.Delete` requires `file_id` and `purpose`, validates both before network
calls, and surfaces MiniMax `base_resp` failures through the shared protocol
error model.
