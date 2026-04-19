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
- `file.go`: file upload API
- `internal/`: transport/protocol/stream/codec internals
- `examples/`: runnable CLI demos
