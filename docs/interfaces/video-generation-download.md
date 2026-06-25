# Video Generation Download

- Official docs: https://platform.minimaxi.com/docs/api-reference/video-generation-download.md
- Endpoint: `GET /v1/files/retrieve`
- SDK status: `Implemented through file management`
- Local code: `file.go`, `file_test.go`.

## Purpose

Retrieve video file download information by `file_id`.

## SDK surface

Use `File.Retrieve(ctx, fileID)` for metadata and download URL details, or
`File.Download(ctx, fileID)` for raw content streams. The SDK does not duplicate
these file endpoints under `VideoService`. `File.Download` handles generated
video files by falling back to the retrieved signed `download_url` when
`retrieve_content` rejects the file purpose.
