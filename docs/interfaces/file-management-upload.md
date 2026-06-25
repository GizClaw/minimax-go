# File Upload

- Official docs: https://platform.minimaxi.com/docs/api-reference/file-management-upload.md
- Endpoint: `POST /v1/files/upload`
- SDK status: `Implemented`
- Local code: `File.Upload` in `file.go`; examples in `examples/file` and `examples/voice/clone`.

## Purpose

Upload a file to MiniMax for downstream APIs such as voice cloning, async T2A
file input, and generated asset workflows.

## Current SDK shape

`FileUploadRequest` supports `Purpose`, `FileName`, `ContentType`, and raw
`Data`. `FileUploadResponse` normalizes `file_id`, `file_url`, upload flag, and
basic metadata.

## Development notes

Keep this as the shared upload primitive. Add typed purpose constants only when
adding higher-level helpers for voice clone, prompt audio, or async T2A text
file upload.

