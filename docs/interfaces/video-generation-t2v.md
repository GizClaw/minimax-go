# Video Generation T2V

- Official docs: https://platform.minimaxi.com/docs/api-reference/video-generation-t2v.md
- Endpoint: `POST /v1/video_generation`
- SDK status: `Implemented`
- Local code: `video.go`, `video_test.go`, `client.go`.

## Purpose

Create an async video generation task from text.

## Key fields

Request fields include `model`, `prompt`, `prompt_optimizer`,
`fast_pretreatment`, `duration`, `resolution`, `callback_url`, and
`aigc_watermark`. Response returns `task_id`.

## SDK surface

```go
type VideoTextToVideoRequest struct {
	Model            string
	Prompt           string
	PromptOptimizer  *bool
	FastPretreatment *bool
	Duration         *int
	Resolution       string
	CallbackURL      string
	AIGCWatermark    *bool
}

func (s *VideoService) CreateTextToVideo(ctx context.Context, request VideoTextToVideoRequest) (*VideoTaskCreateResponse, error)
```

## Implementation notes

The SDK validates required `model` and `prompt` before network calls, preserves
explicit false boolean values with pointer fields, and keeps unknown response
fields in `Raw` for forward compatibility.
