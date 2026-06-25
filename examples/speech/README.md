# Speech Example (Unified)

`examples/speech` is the unified speech CLI entry with subcommands:

- `async`: submit async TTS **or** query existing `task_id` (with `-wait` / `-no-wait`)
- `stream`: stream TTS chunks and merge to a local file
- `websocket`: official WebSocket T2A protocol and merge received audio chunks
- `http`: synchronous HTTP TTS and write local file

`task` is kept as a backward-compatible alias of `async` task query mode.
It is **query-only** and requires explicit `-task-id`.

## Quick start

```bash
export MINIMAX_API_KEY="your_api_key"
```

### 1) Async

```bash
go run ./examples/speech async \
  -text "hello async" \
  -voice-id "male-qn-qingse"
```

### 2) Stream

```bash
go run ./examples/speech stream \
  -text "hello stream" \
  -voice-id "male-qn-qingse" \
  -output /tmp/speech_stream_output.audio
```

### 3) Task Query (inside `async`)

```bash
go run ./examples/speech async \
  -task-id 123456789 \
  -wait
```

### 4) WebSocket

```bash
go run ./examples/speech websocket \
  -text "hello websocket" \
  -voice-id "male-qn-qingse" \
  -output /tmp/speech_websocket_output.audio
```

### 5) HTTP (sync)

```bash
go run ./examples/speech http \
  -text "hello http" \
  -voice-id "male-qn-qingse" \
  -output /tmp/speech_output.audio
```

HTTP, async submit, and WebSocket commands expose the stable official speech fields:

```bash
go run ./examples/speech http \
  -text "hello with explicit audio settings" \
  -voice-id "male-qn-qingse" \
  -language-boost English \
  -output-format url \
  -audio-format mp3 \
  -sample-rate 32000 \
  -bitrate 128000 \
  -channel 1
```

`-output-format` is available for HTTP and async submit. Audio setting flags are available for HTTP, async submit, and WebSocket.

## Show CLI help

```bash
go run ./examples/speech -h
go run ./examples/speech async -h
go run ./examples/speech stream -h
go run ./examples/speech websocket -h
go run ./examples/speech http -h
```

Alias help (deprecated but supported):

```bash
go run ./examples/speech task -h
```

Query-only behavior:

```bash
go run ./examples/speech task -task-id 123456789 -wait
```

If `-task-id` is missing, the command fails fast and will not trigger submit.

## Migration from old example paths

- `go run ./examples/speech_async ...` -> `go run ./examples/speech async ...`
- `go run ./examples/speech_stream ...` -> `go run ./examples/speech stream ...`
- `go run ./examples/speech ...` (old sync mode) -> `go run ./examples/speech http ...`

Backward compatibility is kept for old sync flag style:

```bash
go run ./examples/speech -text "hello" -voice-id "male-qn-qingse"
```

This is treated as the `http` command.
