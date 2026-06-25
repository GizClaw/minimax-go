# Music Cover Preprocess

- Official docs: https://platform.minimaxi.com/docs/api-reference/music-cover-preprocess.md
- Endpoint: `POST /v1/music_cover_preprocess`
- SDK status: `Implemented`
- Local code: `Music.PreprocessCover` in `music.go`; tests in `music_test.go`; example in `examples/music`.

## Purpose

Preprocess reference audio and return a `cover_feature_id` for two-step music
cover workflows.

## Development notes

This should feed `Music.Generate` when using `music-cover` models. Model
`cover_feature_id` expiry explicitly if returned by the API.

The SDK validates that exactly one of `audio_url` or `audio_base64` is supplied.
