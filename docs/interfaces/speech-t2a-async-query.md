# Speech T2A Async Query

- Official docs: https://platform.minimaxi.com/docs/api-reference/speech-t2a-async-query.md
- Endpoint: `GET /v1/query/t2a_async_query_v2`
- SDK status: `Implemented`
- Local code: `SpeechAsync.GetAsyncTask` in `speech_async.go`.

## Purpose

Query a long-text speech synthesis task.

## Current SDK shape

The SDK normalizes task state, `file_id`, URL fields, decoded hex audio when
present, failure information, and selected metadata.

## Current SDK shape

The SDK keeps typed task states, normalizes official and legacy status strings,
preserves raw payload fields, returns file ID and URL variants, decodes hex
audio when present, and maps selected duration, size, format, sample-rate,
bitrate, and channel metadata.
