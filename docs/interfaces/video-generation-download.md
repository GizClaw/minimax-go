# Video Generation Download

- Official docs: https://platform.minimaxi.com/docs/api-reference/video-generation-download.md
- Endpoint: `GET /v1/files/retrieve`
- SDK status: `Not implemented`
- Local code: none.

## Purpose

Retrieve video file download information by `file_id`.

## Development notes

This should likely reuse `File.Retrieve` instead of duplicating a video-specific
download method. Add a convenience helper only after file retrieval exists.

