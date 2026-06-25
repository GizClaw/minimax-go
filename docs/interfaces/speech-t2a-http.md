# Speech T2A HTTP

- Official docs: https://platform.minimaxi.com/docs/api-reference/speech-t2a-http.md
- Endpoint: `POST /v1/t2a_v2`
- SDK status: `Partial`
- Local code: `Speech.Synthesize` in `speech.go`; tests in `speech_test.go`.

## Purpose

Synchronous text-to-audio generation over HTTP.

## Current SDK shape

The SDK sends `model`, `text`, `output_format: hex`, and optional
`voice_setting` fields for `voice_id`, `speed`, `vol`, and `pitch`. It decodes
hex audio into bytes.

## Gaps

The official API supports more audio and language controls than the current SDK
exposes, including broader audio settings and output formats. Keep existing
behavior stable when adding optional fields.

