# Voice Cloning Upload Clone Audio

- Official docs: https://platform.minimaxi.com/docs/api-reference/voice-cloning-uploadcloneaudio.md
- Endpoint: `POST /v1/files/upload`
- SDK status: `Implemented`
- Local code: `Voice.UploadCloneAudio` in `voice.go`, backed by `File.Upload`;
  tests in `voice_test.go`; example use in `examples/voice/clone`.

## Purpose

Upload source audio for voice cloning and receive a `file_id`.

## Current SDK shape

The typed helper validates filename/content, reads the provided `io.Reader`, and
uploads with official purpose `voice_clone`. `File.Upload` remains available for
advanced or custom purpose usage.
