# Music Generation

- Official docs: https://platform.minimaxi.com/docs/api-reference/music-generation.md
- Endpoint: `POST /v1/music_generation`
- SDK status: `Partial`
- Local code: `Music.Generate` in `music.go`; tests in `music_test.go`; example in `examples/music`.

## Purpose

Generate music from a song description and lyrics, or generate cover music from
reference audio/features.

## Key fields

Models include `music-2.6`, `music-cover`, `music-2.6-free`, and
`music-cover-free`. Request fields include `prompt`, `lyrics`, `stream`,
`output_format`, `audio_setting`, `aigc_watermark`, `lyrics_optimizer`,
`is_instrumental`, `audio_url`, `audio_base64`, and `cover_feature_id`.

## Development notes

The SDK implements non-streaming generation for direct songs, instrumental
tracks, one-step cover generation, and two-step cover generation through
`cover_feature_id`.

`stream=true` is rejected locally with a clear unsupported error. Add stream
support only after the official streaming event shape is implemented and tested.
