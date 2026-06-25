# Video Example

`examples/video` demonstrates MiniMax text-to-video, image-to-video,
first-last-frame, and subject-reference video task creation, task query, and the
file retrieve handoff.

## Quick start

```bash
export MINIMAX_API_KEY="your_api_key"

go run ./examples/video \
  -prompt "A small robot paints a glowing green circuit board on a clean desk" \
  -wait
```

## Image-to-video

```bash
go run ./examples/video \
  -first-frame-image https://example.com/frame.png \
  -prompt "Camera pushes in as the subject turns toward the light" \
  -wait
```

When the task succeeds, the command prints the generated `file_id` and any
retrieved `download_url`.

## Subject-reference video

```bash
go run ./examples/video \
  -model S2V-01 \
  -subject-reference character=https://example.com/person.jpg \
  -prompt "The character runs toward the camera and smiles" \
  -wait
```

`-subject-reference` enables subject-reference mode. The current MiniMax S2V
API documents one `character` subject with one image.

## First-last-frame video

```bash
go run ./examples/video \
  -model MiniMax-Hailuo-02 \
  -first-frame-image https://example.com/start.png \
  -last-frame-image https://example.com/end.png \
  -prompt "Camera pulls back as the subject turns toward the light" \
  -wait
```

`-last-frame-image` enables first-last-frame mode. `-first-frame-image` and
`-prompt` are optional for the SDK request, but they are usually useful for
controlling the generated motion.

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
- `-subject-reference`: subject reference as `type=image_url`, for example `character=https://example.com/person.jpg`
- `-first-frame-image`: public image URL or Data URL for image-to-video submit mode
- `-last-frame-image`: public image URL or Data URL for first-last-frame submit mode
- `-task-id`: query an existing task instead of submitting a new one
- `-wait`: poll until the task reaches `success` or `failed`
- `-output`: optional raw video output path after a successful task

## Environment variables

- `MINIMAX_API_KEY`
- `MINIMAX_BASE_URL`
- `MINIMAX_VIDEO_MODEL`
- `MINIMAX_VIDEO_PROMPT`
- `MINIMAX_VIDEO_SUBJECT_REFERENCE`
- `MINIMAX_VIDEO_FIRST_FRAME_IMAGE`
- `MINIMAX_VIDEO_LAST_FRAME_IMAGE`
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
