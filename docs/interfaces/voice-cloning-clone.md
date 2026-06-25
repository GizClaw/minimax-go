# Voice Cloning Clone

- Official docs: https://platform.minimaxi.com/docs/api-reference/voice-cloning-clone.md
- Endpoint: `POST /v1/voice_clone`
- SDK status: `Partial`
- Local code: `Voice.CloneVoice` in `voice.go`; example in `examples/voice/clone`.

## Purpose

Create a cloned voice from uploaded audio or an audio URL.

## Current SDK shape

`CloneVoiceRequest` supports `voice_id`, `audio_url`, and `file_id`. The
response normalizes `voice_id` and demo/trial audio variants.

## Gaps

Audit the current official schema for prompt audio and clone prompt fields.
Existing API should remain source-compatible while adding optional request
fields.

