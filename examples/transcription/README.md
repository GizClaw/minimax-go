# Speech-to-text example

Set `MINIMAX_API_KEY` and supply a containerized audio file:

```bash
export MINIMAX_API_KEY="your_api_key"
go run ./examples/transcription -h
go run ./examples/transcription -file audio.wav
go run ./examples/transcription -file interview.mp3 -language zh -format verbose_json
go run ./examples/transcription -file interview.mp3 -format srt -output subtitles.srt
go run ./examples/transcription -file interview.mp3 -format vtt -output subtitles.vtt
go run ./examples/transcription -file audio.wav -stream
```

`-stream` emits typed events as JSON lines, including the final event and billed
audio duration. Concatenate every event's `delta` to obtain the transcript. A
stream error exits with a nonzero status; output already written may be partial.

`-model` defaults to `asr-1.0`, `-format` to `json`, `-base-url` to
`https://api.minimaxi.com`, and `-timeout` to `10m`. The timeout covers upload and
all response reads. `-output` writes to a file; otherwise output goes to stdout.
JSON output includes response metadata. Subtitle output preserves server bytes.

Supported containers: WAV, AIFF, FLAC, M4A (ALAC), MP3, AAC, Opus and Ogg. Audio
must be at most 50 MiB and 500 seconds. Duration and codec checks occur on the
server; raw PCM is unsupported. An empty language hint enables mixed-language
recognition. Streaming only supports `json`; `verbose_json`, `srt` and `vtt`
activate speaker separation/timestamp alignment and cannot be streamed.
