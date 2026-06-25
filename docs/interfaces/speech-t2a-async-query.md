# Speech T2A Async Query

- Official docs: https://platform.minimaxi.com/docs/api-reference/speech-t2a-async-query.md
- Endpoint: `GET /v1/query/t2a_async_query_v2`
- SDK status: `Partial`
- Local code: `SpeechAsync.GetAsyncTask` in `speech_async.go`.

## Purpose

Query a long-text speech synthesis task.

## Current SDK shape

The SDK normalizes task state, `file_id`, URL fields, decoded hex audio when
present, failure information, and selected metadata.

## Development notes

Keep typed task states. Add any missing official metadata without breaking
existing field names.

