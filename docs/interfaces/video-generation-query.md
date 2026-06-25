# Video Generation Query

- Official docs: https://platform.minimaxi.com/docs/api-reference/video-generation-query.md
- Endpoint: `GET /v1/query/video_generation`
- SDK status: `Not implemented`
- Local code: none.

## Purpose

Query async video generation task status and retrieve the generated `file_id`
when successful.

## Development notes

Use typed task states: processing, success, failed, plus raw fallback. Integrate
with `File.Retrieve`/download once file APIs are implemented.

