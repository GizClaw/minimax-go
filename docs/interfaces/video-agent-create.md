# Video Agent Create

- Official docs: https://platform.minimaxi.com/docs/api-reference/video-agent-create.md
- Endpoint: `POST /v1/video_template_generation`
- SDK status: `Not implemented`
- Local code: none.

## Purpose

Create a video generation task using an official video agent/template.

## Development notes

Model template ID, media inputs, and text inputs explicitly. Keep this separate
from generic video generation because templates have distinct validation.

