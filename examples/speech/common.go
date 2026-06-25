package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	minimax "github.com/GizClaw/minimax-go"
)

const exampleDefaultBaseURL = "https://api.minimax.io"

func ensureOutputDir(outputPath string) error {
	dir := filepath.Dir(outputPath)
	if dir == "." {
		return nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	return nil
}

func envOrDefault(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return defaultValue
}

func envOrDefaultFromKeys(keys []string, defaultValue string) string {
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
	}

	return defaultValue
}

func optionalEnvFloat64(key string) (float64, bool, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return 0, false, nil
	}

	parsed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, true, err
	}

	return parsed, true, nil
}

func optionalEnvFloat64FromKeys(keys ...string) (float64, bool, error) {
	for _, key := range keys {
		raw, ok := os.LookupEnv(key)
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}

		parsed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return 0, true, fmt.Errorf("%s: %w", key, err)
		}

		return parsed, true, nil
	}

	return 0, false, nil
}

func optionalEnvIntFromKeys(keys ...string) (int, bool, error) {
	for _, key := range keys {
		raw, ok := os.LookupEnv(key)
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}

		parsed, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return 0, true, fmt.Errorf("%s: %w", key, err)
		}

		return parsed, true, nil
	}

	return 0, false, nil
}

func optionalEnvBool(key string) (bool, bool, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return false, false, nil
	}

	parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, true, err
	}

	return parsed, true, nil
}

func envDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return defaultValue
	}

	parsed, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || parsed <= 0 {
		return defaultValue
	}

	return parsed
}

func envDurationOrDefaultFromKeys(keys []string, defaultValue time.Duration) time.Duration {
	for _, key := range keys {
		raw, ok := os.LookupEnv(key)
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}

		parsed, err := time.ParseDuration(strings.TrimSpace(raw))
		if err != nil || parsed <= 0 {
			continue
		}

		return parsed
	}

	return defaultValue
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})

	return set
}

func displayTaskState(state minimax.SpeechTaskState) string {
	if strings.TrimSpace(string(state)) == "" {
		return "unknown"
	}

	return string(state)
}

func newSpeechAudioSetting(format string, sampleRate, bitrate, channel *int) *minimax.SpeechAudioSetting {
	format = strings.TrimSpace(format)
	if format == "" && sampleRate == nil && bitrate == nil && channel == nil {
		return nil
	}

	return &minimax.SpeechAudioSetting{
		SampleRate: sampleRate,
		Bitrate:    bitrate,
		Format:     format,
		Channel:    channel,
	}
}

func resolveTaskFailureMessage(resp *minimax.SpeechTaskStatusResponse) string {
	if resp == nil {
		return "task query response is nil"
	}

	failureReason := strings.TrimSpace(resp.ErrorMessage)
	if failureReason == "" {
		failureReason = strings.TrimSpace(resp.RawStatus)
	}
	if failureReason == "" {
		failureReason = "task reached failed state"
	}

	return failureReason
}

func printTaskResult(out io.Writer, response *minimax.SpeechTaskStatusResponse) {
	if response == nil {
		return
	}

	fmt.Fprintf(out, "task result: task_id=%s status=%s raw_status=%q file_id=%s audio_url=%s\n",
		response.TaskID,
		displayTaskState(response.Status),
		response.RawStatus,
		response.Result.FileID,
		response.Result.AudioURL,
	)

	if response.Result.Meta.DurationSeconds != nil {
		fmt.Fprintf(out, "duration_seconds=%.3f\n", *response.Result.Meta.DurationSeconds)
	}
	if response.Result.Meta.SizeBytes != nil {
		fmt.Fprintf(out, "size_bytes=%d\n", *response.Result.Meta.SizeBytes)
	}
	if response.Result.Meta.Format != "" {
		fmt.Fprintf(out, "format=%s\n", response.Result.Meta.Format)
	}
	if response.Result.Meta.SampleRate != nil {
		fmt.Fprintf(out, "sample_rate=%d\n", *response.Result.Meta.SampleRate)
	}
	if response.Result.Meta.Bitrate != nil {
		fmt.Fprintf(out, "bitrate=%d\n", *response.Result.Meta.Bitrate)
	}
	if response.Result.Meta.Channel != nil {
		fmt.Fprintf(out, "channel=%d\n", *response.Result.Meta.Channel)
	}
	if len(response.Result.Audio) > 0 {
		fmt.Fprintf(out, "decoded_audio_bytes=%d\n", len(response.Result.Audio))
	}
}

func waitTask(
	ctx context.Context,
	client *minimax.Client,
	taskID string,
	interval time.Duration,
	out io.Writer,
) (*minimax.SpeechTaskStatusResponse, error) {
	if client == nil {
		return nil, errors.New("client is nil")
	}

	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, errors.New("task_id cannot be empty")
	}

	if interval <= 0 {
		return nil, errors.New("poll interval must be greater than 0")
	}

	if out == nil {
		out = io.Discard
	}

	lastPrintedStatus := minimax.SpeechTaskState("")
	for attempt := 1; ; attempt++ {
		statusResp, err := client.SpeechAsync.GetAsyncTask(ctx, taskID)
		if err != nil {
			return nil, err
		}

		if statusResp.Status != lastPrintedStatus || attempt == 1 {
			fmt.Fprintf(out, "poll #%d: task_id=%s status=%s raw_status=%q\n", attempt, statusResp.TaskID, displayTaskState(statusResp.Status), statusResp.RawStatus)
			lastPrintedStatus = statusResp.Status
		}

		if statusResp.Status.IsTerminal() {
			return statusResp, nil
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
