# File Retrieve

- Official docs: https://platform.minimaxi.com/docs/api-reference/file-management-retrieve.md
- Endpoint: `GET /v1/files/retrieve`
- SDK status: `Not implemented`
- Local code: none.

## Purpose

Retrieve file metadata, including generated file download information when
available.

## Development notes

This is required for async speech and video result workflows. The response
should normalize `file_id`, URL fields, expiry information, size, and content
type while keeping raw metadata.

