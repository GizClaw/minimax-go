# Video Generation I2V

- Official docs: https://platform.minimaxi.com/docs/api-reference/video-generation-i2v.md
- Endpoint: `POST /v1/video_generation`
- SDK status: `Implemented`
- Local code: `video.go`, `video_test.go`, `examples/video/`

## Purpose

Create an async video generation task from an image plus optional prompt.

## Development notes

Implemented as `Client.Video.CreateImageToVideo`.

The SDK reuses the existing video task creation response mapping and transport
behavior. The first-frame input is modeled explicitly as `FirstFrameImage` and
sent through the official `first_frame_image` wire field.

The method validates required `model` and `first_frame_image` fields
client-side, trims string fields consistently with text-to-video, and preserves
optional pointer booleans so explicit `false` values are sent.
