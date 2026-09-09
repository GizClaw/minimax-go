# Speech-to-text

- Official reference: https://platform.minimax.cn/docs/api-reference/speech-to-text
- Contract checked: 2026-09-09
- Endpoint: `POST /v1/speech_to_text` (`multipart/form-data`)
- SDK status: implemented, including every documented response mode
- Implementation: `speech_to_text.go`, `speech_to_text_stream.go`
- Example: `examples/transcription`

## Request contract

`Client.SpeechToText.Transcribe(ctx, SpeechToTextRequest)` handles synchronous
results. `Client.SpeechToText.OpenStream(ctx, SpeechToTextRequest)` handles SSE.
All payloads use explicit structs, with typed model/format enums and documented
fields. The method selects the multipart `stream` flag, avoiding conflicting
request flags and return types.

| SDK field | Wire location | Behavior |
| --- | --- | --- |
| `Model` | `model` form field | Defaults to `SpeechToTextModelASR10` (`asr-1.0`) |
| `File.Name` | `file` filename | Required supported extension; basename sent |
| `File.Data` | `file` binary part | Required audio bytes, at most 50 MiB |
| `ResponseFormat` | `response_format` form field | `json` (default), `verbose_json`, `srt`, `vtt` |
| `Language` | `language` header | Optional BCP-47 hint; empty enables mixed-language recognition |

The documented language hints are `zh`, `yue`, `en`, `ja`, `ko`, `th`, `vi`, `id`,
`ms`, `fil`, `ar`, `tr`, `fr`, `de`, `es`, `it`, `pt`, `pl`, `ru` and `uk`.
Language support remains server-validated to permit future additions.

Supported extensions: `.wav`, `.aiff`, `.flac`, `.m4a` (ALAC), `.mp3`, `.aac`,
`.opus`, `.ogg`. Empty audio, oversized audio, unsupported model/format/extension,
invalid filename/header and incompatible streaming format fail before HTTP.
Actual codecs and the 500-second duration limit are checked by the server. Raw
PCM is unsupported. The SDK neither decodes nor transcodes audio.

## Results and lifecycle

`SpeechToTextResponse` preserves `text`, `duration` (seconds), `trace_id`, and,
for verbose JSON, `n_speakers` plus `segments`. Each `SpeechToTextSegment` has
`id`, `start`, `end`, `speaker`, and `text`. Duration and speaker count are
pointers, distinguishing omission from zero. Silent audio can return empty text,
zero speakers and an empty segment list. SRT/VTT bytes are returned verbatim in
`Subtitles`; timestamps and whitespace are not rewritten.

`SpeechToTextStream.Next` yields `SpeechToTextChunk` with `Index`, `Delta`,
`Finish` and optional `Duration`. Indices must increase consecutively from zero.
Concatenate all deltas, including any final delta. The final event requires a
nonnegative duration; subsequent reads return `io.EOF`. EOF without a finish
event returns `io.ErrUnexpectedEOF`. Malformed events and server errors stop
reading and close the body. The final event also closes the body automatically.
Always defer `Close` in case the consumer stops early. Close is idempotent and
may run concurrently with Next; concurrent Next calls are unsupported.

Requests reuse the configured transport, base URL, auth, default headers and
retry policy. The multipart body is buffered and replayable. Do not mutate audio
bytes until the call returns. Retries apply before a stream opens; events are
never replayed after opening. A configured `ShouldRetry` callback governs retries
in the same way as other SDK endpoints. The supplied context controls upload and
response reads. The default HTTP client timeout is 30 seconds; configure a longer
HTTP timeout and context deadline for longer files.

JSON, subtitle and SSE response content types are checked. `ResponseMeta`
preserves status, headers and request/trace IDs. `*minimax.APIError` exposes
`HTTPStatus`, `StatusMsg`, `RequestID`, `TraceID` and optional typed `Detail`
(`Type`, `Message`, `HTTPCode`) for OpenAI-style errors. Existing `base_resp`
errors remain supported. Context cancellation/deadline errors retain identity
through `errors.Is`.

## SDK example

```go
client, err := minimax.NewClient(minimax.Config{
    APIKey: os.Getenv("MINIMAX_API_KEY"),
    BaseURL: "https://api.minimaxi.com",
    HTTPClient: &http.Client{Timeout: 10 * time.Minute},
})
if err != nil { return err }
audio, err := os.ReadFile("interview.mp3")
if err != nil { return err }
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
defer cancel()
result, err := client.SpeechToText.Transcribe(ctx, minimax.SpeechToTextRequest{
    File: minimax.SpeechToTextFile{Name: "interview.mp3", Data: audio},
    Language: "zh",
    ResponseFormat: minimax.SpeechToTextFormatVerboseJSON,
})
if err != nil { return err }
fmt.Println(result.Text, result.Segments)
```

## Validation

Offline `httptest` tests cover multipart data, every output mode, metadata,
structured/business errors, retry boundaries, context cancellation, deadlines,
stream ordering, malformed events, premature EOF and closing pending reads.
No live paid transcription or recognition-quality test is run by default.
