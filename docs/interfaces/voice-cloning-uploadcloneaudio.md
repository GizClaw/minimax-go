# Voice Cloning Upload Clone Audio

- Official docs: https://platform.minimaxi.com/docs/api-reference/voice-cloning-uploadcloneaudio.md
- Endpoint: `POST /v1/files/upload`
- SDK status: `Partial`
- Local code: generic `File.Upload` in `file.go`.

## Purpose

Upload source audio for voice cloning and receive a `file_id`.

## Current SDK shape

The generic upload API can send audio bytes and a `purpose` value, but there is
no dedicated helper for clone-audio upload semantics.

## Development notes

Add a convenience wrapper only if it removes ambiguity around the official
purpose value, validation, and supported audio formats.

