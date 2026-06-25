# Speech T2A Async Create

- Official docs: https://platform.minimaxi.com/docs/api-reference/speech-t2a-async-create.md
- Endpoint: `POST /v1/t2a_async_v2`
- SDK status: `Partial`
- Local code: `SpeechAsync.SubmitAsync` in `speech_async.go`.

## Purpose

Create a long-text async speech synthesis task.

## Current SDK shape

The SDK supports text submission, model selection, basic voice settings, and
normalizes `task_id`, status, and `file_id` variants.

## Gaps

Audit official request fields before extending: async T2A supports long text,
file input, audio settings, timestamps/subtitles, and richer result metadata.

