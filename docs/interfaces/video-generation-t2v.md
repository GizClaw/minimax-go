# Video Generation T2V

- Official docs: https://platform.minimaxi.com/docs/api-reference/video-generation-t2v.md
- Endpoint: `POST /v1/video_generation`
- SDK status: `Not implemented`
- Local code: none.

## Purpose

Create an async video generation task from text.

## Key fields

Request fields include `model`, `prompt`, `prompt_optimizer`,
`fast_pretreatment`, `duration`, `resolution`, `callback_url`, and
`aigc_watermark`. Response returns `task_id`.

## Development notes

Create a `VideoService` with typed task state and shared query/result structs.

