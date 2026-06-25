# Video Generation Subject Reference

- Official docs: https://platform.minimaxi.com/docs/api-reference/video-generation-s2v.md
- Endpoint: `POST /v1/video_generation`
- SDK status: `Implemented`
- Local code: `video.go`, `video_test.go`, `examples/video/`

## Purpose

Create a video task using a subject reference image plus prompt.

## Implementation notes

Implemented as `Client.Video.CreateSubjectReferenceVideo`.

The official OpenAPI schema currently requires `model` and
`subject_reference`. `prompt`, `prompt_optimizer`, `callback_url`, and
`aigc_watermark` are optional request fields.

The SDK models `subject_reference` as a slice of `VideoSubjectReference` values
to preserve the wire shape. MiniMax currently documents support for one
`character` subject and one image.
