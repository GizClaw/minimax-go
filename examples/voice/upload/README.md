# Voice Upload Example

Upload local audio through the typed voice helper methods.

```bash
export MINIMAX_API_KEY="your_api_key"

go run ./examples/voice/upload \
  -kind clone \
  -input /path/to/source.mp3

go run ./examples/voice/upload \
  -kind prompt \
  -input /path/to/prompt.mp3 \
  -content-type audio/mpeg
```

`-kind clone` uses `Voice.UploadCloneAudio`. `-kind prompt` uses `Voice.UploadPromptAudio`.
