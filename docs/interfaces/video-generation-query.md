# Video Generation Query

- Official docs: https://platform.minimaxi.com/docs/api-reference/video-generation-query.md
- Endpoint: `GET /v1/query/video_generation`
- SDK status: `Implemented`
- Local code: `video.go`, `video_test.go`, `client.go`.

## Purpose

Query async video generation task status and retrieve the generated `file_id`
when successful.

## Key fields

Request query field: `task_id`. Response fields include `task_id`, `status`,
`file_id`, `video_width`, and `video_height`.

## SDK surface

```go
type VideoTaskState string

const (
	VideoTaskStateProcessing VideoTaskState = "processing"
	VideoTaskStateSucceeded  VideoTaskState = "success"
	VideoTaskStateFailed     VideoTaskState = "failed"
)

func (s *VideoService) GetTask(ctx context.Context, taskID string) (*VideoTaskStatusResponse, error)
```

## Implementation notes

The SDK normalizes official states such as `Preparing`, `Queueing`,
`Processing`, `Success`, and `Fail`, while preserving the original value in
`RawStatus`. Successful task file IDs can be passed to `File.Retrieve` or
`File.Download`.
