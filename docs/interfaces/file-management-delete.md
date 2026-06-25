# File Delete

- Official docs: https://platform.minimaxi.com/docs/api-reference/file-management-delete.md
- Endpoint: `POST /v1/files/delete`
- SDK status: `Not implemented`
- Local code: none.

## Purpose

Delete a stored MiniMax file.

## Development notes

Place under `FileService`. Validate `file_id` locally before network calls and
surface MiniMax `base_resp` failures through the shared protocol error model.

