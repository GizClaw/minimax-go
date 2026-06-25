# Voice Cloning Upload Prompt Audio

- Official docs: https://platform.minimaxi.com/docs/api-reference/voice-cloning-uploadprompt.md
- Endpoint: `POST /v1/files/upload`
- SDK status: `Partial`
- Local code: generic `File.Upload` in `file.go`.

## Purpose

Upload optional prompt/example audio used to improve voice clone stability.

## Current SDK shape

The SDK has only the generic upload primitive and no dedicated prompt-audio
helper or typed purpose constant.

## Development notes

Implement together with clone request support for prompt audio, otherwise this
helper has little value by itself.

