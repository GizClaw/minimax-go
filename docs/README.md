# MiniMax API Inventory

Last checked against MiniMax official docs: 2026-06-26.

This directory tracks the official MiniMax API surface against the current
`minimax-go` SDK implementation. Each file under `docs/interfaces/` covers one
official interface page or endpoint and records:

- official endpoint and reference URL
- current SDK status
- local implementation files, if any
- development notes for filling the gap

## Current SDK coverage

Implemented or partially implemented today:

- Speech T2A HTTP: `Speech.Synthesize` in `speech.go`
- Speech T2A HTTP streaming helper: `Speech.OpenStream` in `speech_stream.go`
- Speech T2A async create/query: `SpeechAsync` in `speech_async.go`
- File management: `File.Upload`, `File.List`, `File.Retrieve`, `File.Download`, and `File.Delete` in `file.go`
- Voice list/design/clone: `Voice` in `voice.go`
- Image text-to-image and image-to-image: `Image` in `image.go`
- Video text-to-video, image-to-video create/query: `Video` in `video.go`

Not implemented today:

- Text/OpenAI/Anthropic compatible model calls
- Model list/retrieve endpoints
- Music generation, lyrics generation, music cover preprocess
- Remaining video generation and video agent task APIs: first-last-frame, subject-reference, and video agent tasks
- Voice delete
- Official WebSocket T2A client

## Interface files

### File management

- [file-management-upload.md](interfaces/file-management-upload.md)
- [file-management-list.md](interfaces/file-management-list.md)
- [file-management-retrieve.md](interfaces/file-management-retrieve.md)
- [file-management-retrieve-content.md](interfaces/file-management-retrieve-content.md)
- [file-management-delete.md](interfaces/file-management-delete.md)

### Speech and voice

- [speech-t2a-http.md](interfaces/speech-t2a-http.md)
- [speech-t2a-websocket.md](interfaces/speech-t2a-websocket.md)
- [speech-t2a-async-create.md](interfaces/speech-t2a-async-create.md)
- [speech-t2a-async-query.md](interfaces/speech-t2a-async-query.md)
- [voice-management-get.md](interfaces/voice-management-get.md)
- [voice-management-delete.md](interfaces/voice-management-delete.md)
- [voice-design-design.md](interfaces/voice-design-design.md)
- [voice-cloning-uploadcloneaudio.md](interfaces/voice-cloning-uploadcloneaudio.md)
- [voice-cloning-uploadprompt.md](interfaces/voice-cloning-uploadprompt.md)
- [voice-cloning-clone.md](interfaces/voice-cloning-clone.md)

### Image, music, and video

- [image-generation-t2i.md](interfaces/image-generation-t2i.md)
- [image-generation-i2i.md](interfaces/image-generation-i2i.md)
- [lyrics-generation.md](interfaces/lyrics-generation.md)
- [music-cover-preprocess.md](interfaces/music-cover-preprocess.md)
- [music-generation.md](interfaces/music-generation.md)
- [video-generation-t2v.md](interfaces/video-generation-t2v.md)
- [video-generation-i2v.md](interfaces/video-generation-i2v.md)
- [video-generation-fl2v.md](interfaces/video-generation-fl2v.md)
- [video-generation-s2v.md](interfaces/video-generation-s2v.md)
- [video-generation-query.md](interfaces/video-generation-query.md)
- [video-generation-download.md](interfaces/video-generation-download.md)
- [video-agent-create.md](interfaces/video-agent-create.md)
- [video-agent-query.md](interfaces/video-agent-query.md)

### Text and models

- [responses-create.md](interfaces/responses-create.md)
- [responses-input-tokens.md](interfaces/responses-input-tokens.md)
- [text-chat-openai.md](interfaces/text-chat-openai.md)
- [text-chat-anthropic.md](interfaces/text-chat-anthropic.md)
- [models-openai-list.md](interfaces/models-openai-list.md)
- [models-openai-retrieve.md](interfaces/models-openai-retrieve.md)
- [models-anthropic-list.md](interfaces/models-anthropic-list.md)
- [models-anthropic-retrieve.md](interfaces/models-anthropic-retrieve.md)
