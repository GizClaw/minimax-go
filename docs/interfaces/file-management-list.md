# File List

- Official docs: https://platform.minimaxi.com/docs/api-reference/file-management-list.md
- Endpoint: `GET /v1/files/list`
- SDK status: `Not implemented`
- Local code: none.

## Purpose

List files stored in the MiniMax file system by category and pagination
parameters.

## Development notes

Add this under `FileService` rather than creating a separate service. Model
pagination explicitly and preserve unknown file metadata in a raw payload for
forward compatibility.

