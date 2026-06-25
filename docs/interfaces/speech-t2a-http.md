# Speech T2A HTTP

- Official docs: https://platform.minimaxi.com/docs/api-reference/speech-t2a-http.md
- Endpoint: `POST /v1/t2a_v2`
- SDK status: `Implemented`
- Local code: `Speech.Synthesize` in `speech.go`; tests in `speech_test.go`.

## Purpose

Synchronous text-to-audio generation over HTTP.

## Current SDK shape

The SDK sends `model`, `text`, `stream:false`, `output_format`, optional
`voice_setting`, `audio_setting`, pronunciation, timbre, language, subtitle,
watermark, and voice-modify fields. Hex output is decoded into `Audio`; URL
output is returned as `AudioURL`.

## Notes

Existing callers keep the default `output_format=hex` behavior. Optional fields
are additive and preserve source compatibility.
