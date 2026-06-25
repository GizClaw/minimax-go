# Voice Management Delete

- Official docs: https://platform.minimaxi.com/docs/api-reference/voice-management-delete.md
- Endpoint: `POST /v1/delete_voice`
- SDK status: `Implemented`
- Local code: `Voice.DeleteVoice` in `voice.go`; tests in `voice_test.go`;
  example in `examples/voice/delete`.

## Purpose

Delete generated voice resources that the account owns.

## Current SDK shape

The SDK validates non-empty `voice_id`, validates or defaults `voice_type`,
calls `POST /v1/delete_voice`, returns typed response metadata and the deleted
voice ID, and preserves unrecognized response fields in `Raw`.

`DeleteVoiceRequest.VoiceType` accepts `voice_generation` or `voice_cloning`.
When omitted, it defaults to `voice_generation`, which matches voices created by
`Voice.DesignVoice`.
