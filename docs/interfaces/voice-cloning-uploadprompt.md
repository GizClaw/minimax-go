# Voice Cloning Upload Prompt Audio

- Official docs: https://platform.minimaxi.com/docs/api-reference/voice-cloning-uploadprompt.md
- Endpoint: `POST /v1/files/upload`
- SDK status: `Implemented`
- Local code: `Voice.UploadPromptAudio` in `voice.go`, backed by `File.Upload`;
  tests in `voice_test.go`.

## Purpose

Upload optional prompt/example audio used to improve voice clone stability.

## Current SDK shape

The typed helper validates filename/content, reads the provided `io.Reader`, and
uploads with official purpose `prompt_audio`. `File.Upload` remains available for
advanced or custom purpose usage.
