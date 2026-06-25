# minimax-go

[![Go CI](https://github.com/GizClaw/minimax-go/actions/workflows/go-ci.yml/badge.svg)](https://github.com/GizClaw/minimax-go/actions/workflows/go-ci.yml)
[![CodeQL](https://github.com/GizClaw/minimax-go/actions/workflows/codeql.yml/badge.svg)](https://github.com/GizClaw/minimax-go/actions/workflows/codeql.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/GizClaw/minimax-go)](https://goreportcard.com/report/github.com/GizClaw/minimax-go)

Go SDK and examples for MiniMax APIs.

## What is included

- Speech APIs
  - synchronous HTTP TTS
  - streaming TTS
  - async TTS task submit/query
- File upload API
- Voice APIs
  - list voices
  - voice design
  - voice clone
- Image APIs
  - text-to-image generation
  - image-to-image generation
- Video APIs
  - text-to-video task submit/query
  - image-to-video task submit
  - first-last-frame video task submit

## Roadmap

The detailed API inventory lives in [`docs/`](docs/). Current coverage is:

Implemented:

- [x] File upload: `File.Upload` supports multipart upload and normalized upload metadata.
- [x] File list: `File.List` lists stored files by MiniMax purpose.
- [x] File retrieve: `File.Retrieve` retrieves normalized metadata for a generated or uploaded file.
- [x] File download: `File.Download` opens a raw file content stream for generated files.
- [x] File delete: `File.Delete` deletes a stored file by file ID and purpose.
- [x] Voice list: `Voice.ListVoices` queries available system, cloned, and generated voices.
- [x] Voice design: `Voice.DesignVoice` creates a custom voice from a prompt and preview text.
- [x] Image T2I: `Image.GenerateTextToImage` generates images from text prompts.
- [x] Image I2I: `Image.GenerateImageToImage` generates images from prompts and subject references.
- [x] Video T2V create: `Video.CreateTextToVideo` creates async text-to-video tasks.
- [x] Video I2V create: `Video.CreateImageToVideo` creates async image-to-video tasks.
- [x] Video FL2V create: `Video.CreateFirstLastFrameVideo` creates async first-last-frame video tasks.
- [x] Video generation query: `Video.GetTask` queries async video task status and generated file IDs.

Partially implemented:

- [ ] Speech T2A HTTP: `Speech.Synthesize` supports synchronous HTTP TTS with hex audio output; more official audio/output options still need to be exposed.
- [ ] Speech T2A streaming: `Speech.OpenStream` provides an HTTP stream helper; the official WebSocket T2A protocol is not implemented yet.
- [ ] Speech T2A async: `SpeechAsync.SubmitAsync` and `SpeechAsync.GetAsyncTask` are implemented; some official async fields and metadata still need a schema audit.
- [ ] Voice clone: `Voice.CloneVoice` supports `audio_url` and `file_id`; dedicated prompt-audio helpers and full official clone fields are still missing.
- [ ] Voice clone audio uploads: generic `File.Upload` can upload clone/prompt audio, but there are no dedicated typed helpers yet.

Planned:

- [ ] Voice delete API.
- [ ] Music APIs: lyrics generation, music cover preprocess, and music generation.
- [ ] Remaining video generation APIs: subject-reference and video agent tasks.
- [ ] Text and model APIs: OpenAI/Anthropic-compatible chat, Responses, token estimation, and model list/retrieve endpoints.

## Requirements

- Go `1.26+`
- MiniMax API key

## Quick start

Set your API key:

```bash
export MINIMAX_API_KEY="your_api_key"
```

Check runnable examples:

```bash
go run ./examples/speech -h
go run ./examples/speech async -h
go run ./examples/speech stream -h
go run ./examples/speech http -h
go run ./examples/voice/list -h
go run ./examples/file -h
go run ./examples/image -h
go run ./examples/video -h
```

## Development checks

```bash
go fmt ./...
go build ./...
go vet ./...
go test ./...
```

## Repository layout

- `client.go`: SDK client and service wiring
- `speech*.go`: speech sync/stream/async APIs
- `voice.go`: voice-related APIs
- `image.go`: image generation APIs
- `video.go`: video generation task APIs
- `file.go`: file management APIs
- `docs/`: official API inventory and implementation status by interface
- `internal/`: transport/protocol/stream/codec internals
- `examples/`: runnable CLI demos
