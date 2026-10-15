# Speech T2A Bidirectional Streaming

- Official docs: https://platform.minimax.cn/docs/api-reference/speech-t2a-websocket-bidi
- Endpoint: `WSS /ws/v1/t2a_v2_bidi`
- SDK status: `Implemented`
- Local code: `Speech.OpenBidi` in `speech_bidi.go`

## Ownership and API

The provider buffers incremental text and chooses sentence boundaries. The
client owns event order, downstream audio delivery, turn boundaries and session
lifetime. `SpeechBidiRequest` configures model, voice, language, pronunciation
and audio settings without an initial text field.

| Method | Supported behavior |
| --- | --- |
| `Speech.OpenBidi(ctx, request)` | Connect, wait for `connected_success`, send one `task_start`, and wait for `task_started`. |
| `SendText(ctx, text)` | Send UTF-8 text without trimming or adding punctuation; at most 10,000 Unicode characters per message. |
| `Next(ctx)` | Read an audio, sentence, flush, cancel, or terminal event. One reader may run alongside writers. |
| `Flush(ctx)` | Send `task_flush`; continue reading until `task_flushed` before changing turns. |
| `Cancel(ctx)` | Send `task_cancel`; discard old-turn audio until `task_canceled`, then continue on the same session. |
| `Finish(ctx)` | End input; read remaining audio and `task_finished`. Further text is rejected. |
| `Ping(ctx)` | Keep an idle connection alive; requires the reader to process pong frames. |
| `Close()` | Abort immediately and release pending IO. Graceful completion uses `Finish` first. |

`Flush` and `Cancel` send control events; they do not read or wait for their
acknowledgements. Callers keep a single `Next` loop running. Writes are
serialized, but the caller must order control events after its text producer.
The opening context governs startup; callers must pass their invocation context
to ongoing IO and close the session on cancellation or failure.

## Event and error semantics

`SpeechBidiEvent` preserves `Event`, `SessionID`, `ConnectID`, `TraceID`, audio,
`IsFinal`, and actual `UsageCharacters`. `is_final` ends one audio request,
`sentence_end` ends one provider sentence, and only `task_finished` makes
`Done` true. Neither flush nor cancellation ends the session.

Normal socket closure before `task_finished` is an unexpected EOF. Provider
business errors remain typed API errors. In particular, `2204` (oversized
message) and `2205` (queued text limit) leave the provider session available;
the SDK returns the error without reconnecting or replaying input. Recovery
requires the caller to know which text event failed. Other parse/IO failures
must be handled and the session closed.

The configured HTTP client's transport, proxy, headers and dial timeout are
used for the WS upgrade. Provider messages have a 16 MiB read limit.

The provider discards whitespace-only text events. Keep a space/newline with
neighboring text when chunking, and use flush for an unpunctuated turn tail.
The runnable `examples/speech bidi` and `compare` commands demonstrate both.

## Validation

Offline tests cover TLS/client transport injection, incremental Unicode text,
sentence/final event separation, flush and cancel reuse, terminal handling,
large audio frames, malformed/provider errors, close and cancellation.
Online calls are explicit through the examples. HTTP and ordinary WS APIs
remain separate entry points.
