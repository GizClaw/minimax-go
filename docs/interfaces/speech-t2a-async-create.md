# Speech T2A Async Create

- Official docs: https://platform.minimaxi.com/docs/api-reference/speech-t2a-async-create.md
- Endpoint: `POST /v1/t2a_async_v2`
- SDK status: `Implemented`
- Local code: `SpeechAsync.SubmitAsync` in `speech_async.go`.

## Purpose

Create a long-text async speech synthesis task.

## Current SDK shape

The SDK supports text or `text_file_id` submission, model selection, voice
settings, output format, async audio settings, pronunciation dictionary,
language boost, voice modify settings, subtitle fields, and normalized
`task_id`, status, `file_id`, `task_token`, and usage characters.

## Notes

`text_file_id` is encoded as a JSON number when the provided ID is numeric, and
as a string for opaque IDs.
