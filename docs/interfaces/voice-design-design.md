# Voice Design

- Official docs: https://platform.minimaxi.com/docs/api-reference/voice-design-design.md
- Endpoint: `POST /v1/voice_design`
- SDK status: `Implemented`
- Local code: `Voice.DesignVoice` in `voice.go`; example in `examples/voice/design`.

## Purpose

Generate a custom voice from a natural-language voice description and preview
text.

## Current SDK shape

`DesignVoiceRequest` supports `prompt`, `preview_text`, and optional `voice_id`.
The response normalizes `voice_id` and preview/trial audio variants.

## Development notes

Before adding more fields, re-check the official request schema and maintain
local validation for required prompt and preview text.

