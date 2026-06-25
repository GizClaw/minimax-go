# Video Generation First-Last Frame

- Official docs: https://platform.minimaxi.com/docs/api-reference/video-generation-fl2v.md
- Endpoint: `POST /v1/video_generation`
- SDK status: `Implemented`
- Local code: `video.go`, `video_test.go`, `examples/video/`

## Purpose

Create a video task from first and last frame inputs plus text.

## Implementation notes

Implemented as `Client.Video.CreateFirstLastFrameVideo`.

The official OpenAPI schema currently requires `model` and `last_frame_image`.
`first_frame_image`, `prompt`, `prompt_optimizer`, `duration`, `resolution`,
`callback_url`, and `aigc_watermark` are optional request fields.

The SDK keeps this as a separate request type because first-last-frame
validation differs from normal image-to-video.
