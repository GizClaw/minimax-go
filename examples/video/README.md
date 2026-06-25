# Video Example

`examples/video` demonstrates MiniMax text-to-video task creation, task query,
and the file retrieve handoff.

## Quick start

```bash
export MINIMAX_API_KEY="your_api_key"

go run ./examples/video \
  -prompt "A small robot paints a glowing green circuit board on a clean desk" \
  -wait
```

When the task succeeds, the command prints the generated `file_id` and any
retrieved `download_url`.

## Query an existing task

```bash
go run ./examples/video -task-id 123456789 -wait
```

## Download after success

```bash
go run ./examples/video \
  -prompt "A quiet sunrise over a tiny futuristic workshop" \
  -wait \
  -output /tmp/minimax-video.mp4
```

## Show all CLI options

```bash
go run ./examples/video -h
```

## Common flags

- `-api-key`: MiniMax API key (takes precedence over `MINIMAX_API_KEY`)
- `-base-url`: API endpoint (default: `https://api.minimax.io`)
- `-model`: video model for submit mode
- `-prompt`: text prompt for submit mode
- `-task-id`: query an existing task instead of submitting a new one
- `-wait`: poll until the task reaches `success` or `failed`
- `-output`: optional raw video output path after a successful task

## Environment variables

- `MINIMAX_API_KEY`
- `MINIMAX_BASE_URL`
- `MINIMAX_VIDEO_MODEL`
- `MINIMAX_VIDEO_PROMPT`
- `MINIMAX_VIDEO_TASK_ID`
- `MINIMAX_VIDEO_DURATION`
- `MINIMAX_VIDEO_RESOLUTION`
- `MINIMAX_VIDEO_CALLBACK_URL`
- `MINIMAX_VIDEO_PROMPT_OPTIMIZER`
- `MINIMAX_VIDEO_FAST_PRETREATMENT`
- `MINIMAX_VIDEO_AIGC_WATERMARK`
- `MINIMAX_VIDEO_WAIT`
- `MINIMAX_VIDEO_OUTPUT`
- `MINIMAX_VIDEO_TIMEOUT`
- `MINIMAX_VIDEO_POLL_INTERVAL`
