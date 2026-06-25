# Voice Clone Example

`examples/voice/clone` demonstrates `Voice.CloneVoice`.

Supported source inputs:

- `-audio-url`
- `-file-id`
- `-input` (local file upload first through `Voice.UploadCloneAudio`, then clone by returned `file_id`)

## Quick start

### Clone with file_id

```bash
export MINIMAX_API_KEY="your_api_key"

go run ./examples/voice/clone \
  -voice-id "MyCloneVoice001" \
  -file-id "372103253696905"
```

### Clone with local upload

```bash
go run ./examples/voice/clone \
  -voice-id "MyCloneVoice001" \
  -input ./sample.wav \
  -content-type audio/wav
```

### Clone with audio URL

```bash
go run ./examples/voice/clone \
  -voice-id "MyCloneVoice001" \
  -audio-url "https://example.com/sample.wav"
```

> Note: in China region (`https://api.minimax.chat`), `audio_url` clone may be unsupported. Prefer `-file-id` or `-input`.

Use `-json` to print structured output.

## Show all CLI options

```bash
go run ./examples/voice/clone -h
```

## Common flags

- `-api-key`: Minimax API key (takes precedence over `MINIMAX_API_KEY`)
- `-base-url`: API endpoint (default: `https://api.minimax.io`)
- `-voice-id`: target cloned voice ID
- `-audio-url`: source audio URL
- `-file-id`: source uploaded file ID
- `-input`: local file path for upload-then-clone
- `-file-name`: uploaded file name override for `-input`
- `-content-type`: MIME type override for `-input` upload
- `-purpose`: upload purpose (default: `voice_clone`; default path uses `Voice.UploadCloneAudio`)
- `-timeout`: request timeout (default: `30s`)
- `-json`: print response as formatted JSON

## Environment variables

- `MINIMAX_API_KEY`
- `MINIMAX_BASE_URL`
- `MINIMAX_VOICE_CLONE_VOICE_ID`
- `MINIMAX_VOICE_CLONE_AUDIO_URL`
- `MINIMAX_VOICE_CLONE_FILE_ID`
- `MINIMAX_VOICE_CLONE_FILE_INPUT`
- `MINIMAX_VOICE_CLONE_FILE_NAME`
- `MINIMAX_VOICE_CLONE_CONTENT_TYPE`
- `MINIMAX_VOICE_CLONE_PURPOSE`
- `MINIMAX_VOICE_CLONE_TIMEOUT`
