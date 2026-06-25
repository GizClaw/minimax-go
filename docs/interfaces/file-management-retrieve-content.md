# File Download

- Official docs: https://platform.minimaxi.com/docs/api-reference/file-management-retrieve-content.md
- Endpoint: `GET /v1/files/retrieve_content`
- SDK status: `Not implemented`
- Local code: none.

## Purpose

Download file bytes from MiniMax storage.

## Development notes

Expose a streaming `io.ReadCloser` path and a convenience byte-returning helper.
Respect context cancellation and return response metadata/content type.

