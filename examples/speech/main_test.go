package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	minimax "github.com/GizClaw/minimax-go"
)

func TestWaitTask(t *testing.T) {
	t.Parallel()

	t.Run("polls until succeeded", testWaitTaskPollsUntilSucceeded)
	t.Run("input validation", testWaitTaskInputValidation)
	t.Run("context cancel is preserved", testWaitTaskContextCancel)
}

func testWaitTaskPollsUntilSucceeded(t *testing.T) {
	t.Parallel()

	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/query/t2a_async_query_v2" {
			t.Fatalf("path = %s, want /v1/query/t2a_async_query_v2", r.URL.Path)
		}

		if got := r.URL.Query().Get("task_id"); got != "task_1" {
			t.Fatalf("query.task_id = %q, want task_1", got)
		}

		w.Header().Set("Content-Type", "application/json")
		if atomic.AddInt32(&attempts, 1) == 1 {
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"task_id":"task_1","status":"processing"}`))
			return
		}

		_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"task_id":"task_1","status":"success","result":{"audio_url":"https://cdn.example.com/task_1.mp3"}}`))
	}))
	defer srv.Close()

	client, err := minimax.NewClient(minimax.Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var output bytes.Buffer
	resp, err := waitTask(ctx, client, "task_1", time.Millisecond, &output)
	if err != nil {
		t.Fatalf("waitTask() error = %v, want nil", err)
	}

	if resp.Status != minimax.SpeechTaskStateSucceeded {
		t.Fatalf("resp.Status = %q, want succeeded", resp.Status)
	}

	if resp.Result.AudioURL != "https://cdn.example.com/task_1.mp3" {
		t.Fatalf("resp.Result.AudioURL = %q, want https://cdn.example.com/task_1.mp3", resp.Result.AudioURL)
	}

	printed := output.String()
	if !strings.Contains(printed, "poll #1") || !strings.Contains(printed, "poll #2") {
		t.Fatalf("waitTask() output = %q, want poll #1 and poll #2 logs", printed)
	}
}

func testWaitTaskInputValidation(t *testing.T) {
	t.Parallel()

	_, err := waitTask(context.Background(), nil, "task_1", time.Second, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "client is nil") {
		t.Fatalf("waitTask(nil client) error = %v, want client is nil", err)
	}

	client, err := minimax.NewClient(minimax.Config{BaseURL: "https://api.minimax.io"})
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	_, err = waitTask(context.Background(), client, "", time.Second, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "task_id") {
		t.Fatalf("waitTask(empty task id) error = %v, want task_id validation", err)
	}

	_, err = waitTask(context.Background(), client, "task_1", 0, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "poll interval") {
		t.Fatalf("waitTask(zero interval) error = %v, want poll interval validation", err)
	}
}

func testWaitTaskContextCancel(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"base_resp":{"status_code":0,"status_msg":"ok"},"task_id":"task_1","status":"processing"}`))
	}))
	defer srv.Close()

	client, err := minimax.NewClient(minimax.Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = waitTask(ctx, client, "task_1", time.Millisecond, &bytes.Buffer{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waitTask() error = %v, want context canceled", err)
	}
}

func TestOptionalEnvBool(t *testing.T) {
	const envKey = "MINIMAX_SPEECH_ASYNC_NO_WAIT_TEST"

	t.Run("unset and empty are treated as not set", func(t *testing.T) {
		t.Setenv(envKey, "")
		got, set, err := optionalEnvBool(envKey)
		if err != nil {
			t.Fatalf("optionalEnvBool() error = %v, want nil", err)
		}
		if set {
			t.Fatal("optionalEnvBool() set = true, want false")
		}
		if got {
			t.Fatal("optionalEnvBool() value = true, want false")
		}
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		t.Setenv(envKey, "not-bool")
		_, set, err := optionalEnvBool(envKey)
		if err == nil {
			t.Fatal("optionalEnvBool() error = nil, want non-nil")
		}
		if !set {
			t.Fatal("optionalEnvBool() set = false, want true")
		}
	})

	t.Run("valid value", func(t *testing.T) {
		t.Setenv(envKey, "true")
		got, set, err := optionalEnvBool(envKey)
		if err != nil {
			t.Fatalf("optionalEnvBool() error = %v, want nil", err)
		}
		if !set {
			t.Fatal("optionalEnvBool() set = false, want true")
		}
		if !got {
			t.Fatal("optionalEnvBool() value = false, want true")
		}
	})
}

func TestEnvDurationOrDefaultFromKeys(t *testing.T) {
	t.Run("returns default when no key is set", func(t *testing.T) {
		got := envDurationOrDefaultFromKeys([]string{"UNSET_A", "UNSET_B"}, 7*time.Second)
		if got != 7*time.Second {
			t.Fatalf("envDurationOrDefaultFromKeys() = %v, want 7s", got)
		}
	})

	t.Run("skips invalid and picks next valid key", func(t *testing.T) {
		t.Setenv("MINIMAX_ASYNC_DURATION_A", "bad-duration")
		t.Setenv("MINIMAX_ASYNC_DURATION_B", "9s")

		got := envDurationOrDefaultFromKeys([]string{"MINIMAX_ASYNC_DURATION_A", "MINIMAX_ASYNC_DURATION_B"}, 5*time.Second)
		if got != 9*time.Second {
			t.Fatalf("envDurationOrDefaultFromKeys() = %v, want 9s", got)
		}
	})
}

func TestDisplayTaskState(t *testing.T) {
	t.Parallel()

	if got := displayTaskState(""); got != "unknown" {
		t.Fatalf("displayTaskState(empty) = %q, want unknown", got)
	}

	if got := displayTaskState(minimax.SpeechTaskStateQueued); got != "queued" {
		t.Fatalf("displayTaskState(queued) = %q, want queued", got)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run([]string{"unknown"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("run() error = nil, want non-nil")
	}

	if !strings.Contains(err.Error(), "unknown speech command") {
		t.Fatalf("run() error = %v, want unknown command error", err)
	}
}

func TestRunTaskAliasHelp(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run([]string{"task", "-h"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run(task -h) error = %v, want nil", err)
	}

	if !strings.Contains(stderr.String(), "Usage: go run ./examples/speech task") {
		t.Fatalf("task alias help = %q, want task usage", stderr.String())
	}
}

func TestTaskAliasRequiresTaskID(t *testing.T) {
	var submitHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/t2a_async_v2" {
			atomic.AddInt32(&submitHits, 1)
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	t.Setenv("MINIMAX_API_KEY", "dummy-key")
	t.Setenv("MINIMAX_SPEECH_ASYNC_TEXT", "should-not-trigger-submit")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run([]string{"task", "-base-url", srv.URL}, &stdout, &stderr)
	if err == nil {
		t.Fatal("run(task without task-id) error = nil, want non-nil")
	}

	if !strings.Contains(err.Error(), "task command requires -task-id") {
		t.Fatalf("run(task without task-id) error = %v, want missing task-id error", err)
	}

	if !strings.Contains(stderr.String(), "Usage: go run ./examples/speech task") {
		t.Fatalf("task alias usage output = %q, want task usage", stderr.String())
	}

	if atomic.LoadInt32(&submitHits) != 0 {
		t.Fatalf("submit endpoint hits = %d, want 0", submitHits)
	}
}

func TestParseAsyncOptionsWaitConflict(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	_, err := parseAsyncOptions([]string{"-wait", "-no-wait"}, &output)
	if err == nil {
		t.Fatal("parseAsyncOptions() error = nil, want non-nil")
	}

	if !strings.Contains(err.Error(), "cannot be set together") {
		t.Fatalf("parseAsyncOptions() error = %v, want wait/no-wait conflict", err)
	}
}
