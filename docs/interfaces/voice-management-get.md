# Voice Management Get

- Official docs: https://platform.minimaxi.com/docs/api-reference/voice-management-get.md
- Endpoint: `POST /v1/get_voice`
- SDK status: `Implemented`
- Local code: `Voice.ListVoices` in `voice.go`; tests in `voice_test.go`.

## Purpose

Query available system, cloned, and generated voice IDs.

## Current SDK shape

`ListVoicesRequest` supports `voice_type`, `page_size`, and `page_token`.
`ListVoicesResponse` normalizes multiple historical response shapes.

## Development notes

Keep raw payload preservation. Add new official voice fields as optional fields
only when needed by callers.

