# Speech Example (Unified)

`examples/speech` is the unified speech CLI entry with subcommands:

- `async`: submit async TTS **or** query existing `task_id` (with `-wait` / `-no-wait`)
- `stream`: stream TTS chunks and merge to a local file
- `websocket`: official WebSocket T2A protocol and merge received audio chunks
- `bidi`: send incremental text to a persistent session with server sentence buffering
- `compare`: compare HTTP streaming, ordinary WS and a reused bidi session
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

The HTTP/SSE stream writes incremental audio without downloading or replaying
the terminal aggregate copy. The library exposes final billed character metadata
through `SpeechChunk.UsageCharacters`. The CLI's `-timeout` still bounds the turn;
the default SDK stream otherwise lasts until its caller context ends or it closes.

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
go run ./examples/speech bidi -h
go run ./examples/speech compare -h
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

## Bidirectional text streaming

The `bidi` example sends UTF-8 fragments while reading audio concurrently. It
flushes the last incomplete sentence, reads `task_flushed`, then gracefully
finishes the session. Whitespace stays attached to neighboring text fragments
because the provider discards whitespace-only messages. Output is raw PCM16LE,
16 kHz mono; the JSON report records input completion, connection/task startup,
first audio/non-silent chunk arrival, sentence events, audio duration and billing.
The non-silent metric is a sample amplitude threshold, not ASR word alignment or
physical speaker output.

```bash
export MINIMAX_API_KEY="your_api_key"
go run ./examples/speech bidi \
  -base-url https://api.minimaxi.com \
  -model speech-2.6-turbo -voice-id female-shaonv \
  -text '她说：“等我攒够1,000元，就买票回来。”' \
  -chunk-runes 4 -chunk-delay 80ms \
  -output /tmp/bidi.pcm -report /tmp/bidi.json
```

`-cancel-first` cancels a short prefix before streaming the full text again.
`-cancel-after-audio` waits for actual audio before cancelling. Both wait for
`task_canceled`, discard old-turn audio up to that acknowledgement, and resume
on the same connection. They are demonstrations, not timing-comparison modes.
Never put a real key on the command line; use `MINIMAX_API_KEY`.

## Reproducible protocol comparison

```bash
go run ./examples/speech compare \
  -base-url https://api.minimaxi.com \
  -model speech-2.6-turbo -voice-id female-shaonv \
  -text-file /path/to/chinese-quotes.txt \
  -chunk-runes 4 -chunk-delay 0s -iterations 3 \
  -output /tmp/tts-compare-ready
```

Repeat with `-chunk-delay 80ms` and another output directory to simulate text
arriving from an LLM. Protocol order rotates. Each protocol has a separate
transport pool, so a WS upgrade cannot consume an HTTP idle connection. HTTP
reuses its transport; the ordinary WS SDK call creates a session per turn;
bidi uses one connection and `task_flush` between turns. The first bidi turn
includes connection/task startup in the timer; later turns explicitly report
session reuse.

When delay is zero, every protocol starts with the same ready text. When delay
is nonzero, HTTP/ordinary WS wait for the full simulated text, whereas bidi
forwards chunks as they become available. That measures those raw SDK usage
patterns, not GizClaw's client-segmented batching pipeline. It does not establish
an inherent speed ranking of HTTP and WS.

The output directory contains each turn's PCM and `comparison.json`, including
per-event arrival times. All examples use the requested model and voice;
`compare` does not synthesize a hidden warmup. Real cloud calls incur usage and
are only made when this example is invoked with a credential.
