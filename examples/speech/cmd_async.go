package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	minimax "github.com/GizClaw/minimax-go"
)

const (
	asyncDefaultModel        = "speech-2.6-hd"
	asyncDefaultText         = "hello from minimax-go speech async example"
	asyncDefaultTimeout      = 3 * time.Minute
	asyncDefaultPollInterval = 2 * time.Second
)

type asyncOptions struct {
	apiKey        string
	baseURL       string
	taskID        string
	text          string
	textFileID    string
	model         string
	voiceID       string
	speed         *float64
	volume        *float64
	languageBoost string
	outputFormat  string
	audioFormat   string
	sampleRate    *int
	bitrate       *int
	channel       *int
	timeout       time.Duration
	pollInterval  time.Duration
	wait          bool
	noWait        bool
}

type asyncDefaults struct {
	apiKey             string
	baseURL            string
	taskID             string
	text               string
	textFileID         string
	model              string
	voiceID            string
	speed              float64
	volume             float64
	speedSetByEnv      bool
	volumeSetByEnv     bool
	languageBoost      string
	outputFormat       string
	audioFormat        string
	sampleRate         int
	bitrate            int
	channel            int
	sampleRateSetByEnv bool
	bitrateSetByEnv    bool
	channelSetByEnv    bool
	timeout            time.Duration
	pollInterval       time.Duration
	wait               bool
	noWaitFlagDefault  bool
}

func runAsyncCommand(args []string, stdout, stderr io.Writer) error {
	opts, err := parseAsyncOptions(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("failed to parse async flags: %w", err)
	}

	return runAsync(opts, stdout)
}

func runTaskAliasCommand(args []string, stdout, stderr io.Writer) error {
	opts, err := parseTaskAliasOptions(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("failed to parse task flags: %w", err)
	}

	return runAsync(opts, stdout)
}

func parseTaskAliasOptions(args []string, out io.Writer) (asyncOptions, error) {
	var opts asyncOptions

	apiKeyDefault := os.Getenv("MINIMAX_API_KEY")
	baseURLDefault := envOrDefault("MINIMAX_BASE_URL", exampleDefaultBaseURL)
	timeoutDefault := envDurationOrDefaultFromKeys(
		[]string{"MINIMAX_SPEECH_TASK_TIMEOUT", "MINIMAX_SPEECH_ASYNC_TIMEOUT", "MINIMAX_SPEECH_TIMEOUT"},
		asyncDefaultTimeout,
	)
	pollIntervalDefault := envDurationOrDefaultFromKeys(
		[]string{"MINIMAX_SPEECH_TASK_POLL_INTERVAL", "MINIMAX_SPEECH_ASYNC_POLL_INTERVAL"},
		asyncDefaultPollInterval,
	)

	waitDefault, err := resolveAsyncWaitDefault()
	if err != nil {
		return asyncOptions{}, err
	}

	fs := flag.NewFlagSet("task", flag.ContinueOnError)
	fs.SetOutput(out)

	fs.StringVar(&opts.apiKey, "api-key", apiKeyDefault, "Minimax API key (or env MINIMAX_API_KEY)")
	fs.StringVar(&opts.baseURL, "base-url", baseURLDefault, "Minimax API base URL (env: MINIMAX_BASE_URL)")
	fs.StringVar(&opts.taskID, "task-id", "", "Task ID to query (required)")
	fs.DurationVar(&opts.timeout, "timeout", timeoutDefault, "Total timeout for query/watch workflow")
	fs.DurationVar(&opts.pollInterval, "poll-interval", pollIntervalDefault, "Polling interval when wait=true")
	fs.BoolVar(&opts.wait, "wait", waitDefault, "Wait/poll until terminal status")
	fs.BoolVar(&opts.noWait, "no-wait", !waitDefault, "Alias of -wait=false")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: go run ./examples/speech task -task-id <id> [flags]\n\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nNotes:\n")
		fmt.Fprintf(fs.Output(), "  - task is a query-only alias and cannot submit new text\n")
	}

	if err := fs.Parse(args); err != nil {
		return asyncOptions{}, err
	}

	if err := applyAsyncWaitFlags(fs, &opts); err != nil {
		return asyncOptions{}, err
	}

	trimAsyncOptions(&opts)

	if opts.timeout <= 0 {
		return asyncOptions{}, errors.New("timeout must be greater than 0")
	}

	if opts.pollInterval <= 0 {
		return asyncOptions{}, errors.New("poll-interval must be greater than 0")
	}

	if !flagWasSet(fs, "task-id") || opts.taskID == "" {
		fs.Usage()
		return asyncOptions{}, errors.New("task command requires -task-id")
	}

	// Ensure task alias remains query-only.
	opts.text = ""
	opts.textFileID = ""

	return opts, nil
}

func parseAsyncOptions(args []string, out io.Writer) (asyncOptions, error) {
	var opts asyncOptions
	defaults, err := loadAsyncDefaults()
	if err != nil {
		return asyncOptions{}, err
	}

	fs := flag.NewFlagSet("async", flag.ContinueOnError)
	fs.SetOutput(out)

	speedValue := defaults.speed
	volumeValue := defaults.volume

	fs.StringVar(&opts.apiKey, "api-key", defaults.apiKey, "Minimax API key (or env MINIMAX_API_KEY)")
	fs.StringVar(&opts.baseURL, "base-url", defaults.baseURL, "Minimax API base URL (env: MINIMAX_BASE_URL)")
	fs.StringVar(&opts.taskID, "task-id", defaults.taskID, "Query existing task_id instead of submitting new text")
	fs.StringVar(&opts.text, "text", defaults.text, "Input text for submit mode (env: MINIMAX_SPEECH_ASYNC_TEXT)")
	fs.StringVar(&opts.textFileID, "text-file-id", defaults.textFileID, "Uploaded text file_id for submit mode (optional)")
	fs.StringVar(&opts.model, "model", defaults.model, "Model name for submit mode (optional)")
	fs.StringVar(&opts.voiceID, "voice-id", defaults.voiceID, "Voice ID for submit mode (optional; some regions require this)")
	fs.Float64Var(&speedValue, "speed", defaults.speed, "Speech speed for submit mode (optional)")
	fs.Float64Var(&volumeValue, "volume", defaults.volume, "Speech volume for submit mode (optional)")
	fs.StringVar(&opts.languageBoost, "language-boost", defaults.languageBoost, "Language boost value for submit mode, e.g. English or auto")
	fs.StringVar(&opts.outputFormat, "output-format", defaults.outputFormat, "Output format for submit mode: hex or url")
	fs.StringVar(&opts.audioFormat, "audio-format", defaults.audioFormat, "Audio format for submit mode, e.g. mp3/wav/flac")
	sampleRateValue := defaults.sampleRate
	bitrateValue := defaults.bitrate
	channelValue := defaults.channel
	fs.IntVar(&sampleRateValue, "sample-rate", defaults.sampleRate, "Audio sample rate for submit mode")
	fs.IntVar(&bitrateValue, "bitrate", defaults.bitrate, "Audio bitrate for submit mode")
	fs.IntVar(&channelValue, "channel", defaults.channel, "Audio channel count for submit mode")
	fs.DurationVar(&opts.timeout, "timeout", defaults.timeout, "Total timeout for submit/query workflow")
	fs.DurationVar(&opts.pollInterval, "poll-interval", defaults.pollInterval, "Polling interval when wait=true")
	fs.BoolVar(&opts.wait, "wait", defaults.wait, "Wait/poll until terminal status")
	fs.BoolVar(&opts.noWait, "no-wait", defaults.noWaitFlagDefault, "Alias of -wait=false")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: go run ./examples/speech async [flags]\n\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nModes:\n")
		fmt.Fprintf(fs.Output(), "  - submit mode: no -task-id, uses -text/-text-file-id\n")
		fmt.Fprintf(fs.Output(), "  - task mode: set -task-id, query existing task\n")
	}

	if err := fs.Parse(args); err != nil {
		return asyncOptions{}, err
	}

	if err := applyAsyncWaitFlags(fs, &opts); err != nil {
		return asyncOptions{}, err
	}

	trimAsyncOptions(&opts)

	if opts.timeout <= 0 {
		return asyncOptions{}, errors.New("timeout must be greater than 0")
	}

	if opts.pollInterval <= 0 {
		return asyncOptions{}, errors.New("poll-interval must be greater than 0")
	}

	if opts.taskID == "" && opts.text == "" && opts.textFileID == "" {
		return asyncOptions{}, errors.New("submit mode requires text or text-file-id")
	}

	if defaults.speedSetByEnv || flagWasSet(fs, "speed") {
		speed := speedValue
		opts.speed = &speed
	}

	if defaults.volumeSetByEnv || flagWasSet(fs, "volume") {
		volume := volumeValue
		opts.volume = &volume
	}
	if defaults.sampleRateSetByEnv || flagWasSet(fs, "sample-rate") {
		sampleRate := sampleRateValue
		opts.sampleRate = &sampleRate
	}
	if defaults.bitrateSetByEnv || flagWasSet(fs, "bitrate") {
		bitrate := bitrateValue
		opts.bitrate = &bitrate
	}
	if defaults.channelSetByEnv || flagWasSet(fs, "channel") {
		channel := channelValue
		opts.channel = &channel
	}

	return opts, nil
}

func loadAsyncDefaults() (asyncDefaults, error) {
	defaults := asyncDefaults{
		apiKey:        os.Getenv("MINIMAX_API_KEY"),
		baseURL:       envOrDefault("MINIMAX_BASE_URL", exampleDefaultBaseURL),
		taskID:        strings.TrimSpace(envOrDefaultFromKeys([]string{"MINIMAX_SPEECH_TASK_ID", "MINIMAX_SPEECH_ASYNC_TASK_ID"}, "")),
		text:          envOrDefaultFromKeys([]string{"MINIMAX_SPEECH_ASYNC_TEXT", "MINIMAX_SPEECH_TEXT"}, asyncDefaultText),
		textFileID:    os.Getenv("MINIMAX_SPEECH_ASYNC_TEXT_FILE_ID"),
		model:         envOrDefaultFromKeys([]string{"MINIMAX_SPEECH_ASYNC_MODEL", "MINIMAX_SPEECH_MODEL"}, asyncDefaultModel),
		voiceID:       envOrDefaultFromKeys([]string{"MINIMAX_SPEECH_ASYNC_VOICE_ID", "MINIMAX_SPEECH_VOICE_ID"}, ""),
		languageBoost: envOrDefaultFromKeys([]string{"MINIMAX_SPEECH_ASYNC_LANGUAGE_BOOST", "MINIMAX_SPEECH_LANGUAGE_BOOST"}, ""),
		outputFormat:  envOrDefaultFromKeys([]string{"MINIMAX_SPEECH_ASYNC_OUTPUT_FORMAT", "MINIMAX_SPEECH_OUTPUT_FORMAT"}, ""),
		audioFormat:   envOrDefaultFromKeys([]string{"MINIMAX_SPEECH_ASYNC_AUDIO_FORMAT", "MINIMAX_SPEECH_AUDIO_FORMAT"}, ""),
		timeout:       envDurationOrDefaultFromKeys([]string{"MINIMAX_SPEECH_ASYNC_TIMEOUT", "MINIMAX_SPEECH_TIMEOUT"}, asyncDefaultTimeout),
		pollInterval:  envDurationOrDefaultFromKeys([]string{"MINIMAX_SPEECH_ASYNC_POLL_INTERVAL", "MINIMAX_SPEECH_TASK_POLL_INTERVAL"}, asyncDefaultPollInterval),
	}

	speed, speedSetByEnv, err := optionalEnvFloat64FromKeys("MINIMAX_SPEECH_ASYNC_SPEED", "MINIMAX_SPEECH_SPEED")
	if err != nil {
		return asyncDefaults{}, fmt.Errorf("invalid speech speed env: %w", err)
	}
	if !speedSetByEnv {
		speed = 1
	}
	defaults.speed = speed
	defaults.speedSetByEnv = speedSetByEnv

	volume, volumeSetByEnv, err := optionalEnvFloat64FromKeys("MINIMAX_SPEECH_ASYNC_VOLUME", "MINIMAX_SPEECH_VOLUME")
	if err != nil {
		return asyncDefaults{}, fmt.Errorf("invalid speech volume env: %w", err)
	}
	if !volumeSetByEnv {
		volume = 1
	}
	defaults.volume = volume
	defaults.volumeSetByEnv = volumeSetByEnv

	sampleRate, sampleRateSetByEnv, err := optionalEnvIntFromKeys("MINIMAX_SPEECH_ASYNC_SAMPLE_RATE", "MINIMAX_SPEECH_SAMPLE_RATE")
	if err != nil {
		return asyncDefaults{}, fmt.Errorf("invalid speech sample rate env: %w", err)
	}
	defaults.sampleRate = sampleRate
	defaults.sampleRateSetByEnv = sampleRateSetByEnv

	bitrate, bitrateSetByEnv, err := optionalEnvIntFromKeys("MINIMAX_SPEECH_ASYNC_BITRATE", "MINIMAX_SPEECH_BITRATE")
	if err != nil {
		return asyncDefaults{}, fmt.Errorf("invalid speech bitrate env: %w", err)
	}
	defaults.bitrate = bitrate
	defaults.bitrateSetByEnv = bitrateSetByEnv

	channel, channelSetByEnv, err := optionalEnvIntFromKeys("MINIMAX_SPEECH_ASYNC_CHANNEL", "MINIMAX_SPEECH_CHANNEL")
	if err != nil {
		return asyncDefaults{}, fmt.Errorf("invalid speech channel env: %w", err)
	}
	defaults.channel = channel
	defaults.channelSetByEnv = channelSetByEnv

	waitDefault, err := resolveAsyncWaitDefault()
	if err != nil {
		return asyncDefaults{}, err
	}
	defaults.wait = waitDefault
	defaults.noWaitFlagDefault = !waitDefault

	return defaults, nil
}

func resolveAsyncWaitDefault() (bool, error) {
	waitDefault := true
	if waitFromEnv, waitSetByEnv, waitErr := optionalEnvBool("MINIMAX_SPEECH_ASYNC_WAIT"); waitErr != nil {
		return false, fmt.Errorf("invalid MINIMAX_SPEECH_ASYNC_WAIT: %w", waitErr)
	} else if waitSetByEnv {
		waitDefault = waitFromEnv
	}

	if noWaitFromEnv, noWaitSetByEnv, noWaitErr := optionalEnvBool("MINIMAX_SPEECH_ASYNC_NO_WAIT"); noWaitErr != nil {
		return false, fmt.Errorf("invalid MINIMAX_SPEECH_ASYNC_NO_WAIT: %w", noWaitErr)
	} else if noWaitSetByEnv {
		waitDefault = !noWaitFromEnv
	}

	return waitDefault, nil
}

func applyAsyncWaitFlags(fs *flag.FlagSet, opts *asyncOptions) error {
	if flagWasSet(fs, "wait") && flagWasSet(fs, "no-wait") {
		return errors.New("wait and no-wait cannot be set together")
	}
	if flagWasSet(fs, "no-wait") {
		opts.wait = !opts.noWait
	}

	return nil
}

func trimAsyncOptions(opts *asyncOptions) {
	opts.apiKey = strings.TrimSpace(opts.apiKey)
	opts.baseURL = strings.TrimSpace(opts.baseURL)
	opts.taskID = strings.TrimSpace(opts.taskID)
	opts.text = strings.TrimSpace(opts.text)
	opts.textFileID = strings.TrimSpace(opts.textFileID)
	opts.model = strings.TrimSpace(opts.model)
	opts.voiceID = strings.TrimSpace(opts.voiceID)
	opts.languageBoost = strings.TrimSpace(opts.languageBoost)
	opts.outputFormat = strings.TrimSpace(opts.outputFormat)
	opts.audioFormat = strings.TrimSpace(opts.audioFormat)
}

func runAsync(opts asyncOptions, out io.Writer) error {
	if opts.apiKey == "" {
		return errors.New("missing API key: use -api-key or set MINIMAX_API_KEY")
	}

	if opts.baseURL == "" {
		return errors.New("base-url cannot be empty")
	}

	client, err := minimax.NewClient(minimax.Config{
		BaseURL: opts.baseURL,
		APIKey:  opts.apiKey,
	})
	if err != nil {
		return fmt.Errorf("failed to create Minimax client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()

	if opts.taskID != "" {
		return runAsyncTaskMode(ctx, client, opts, out)
	}

	return runAsyncSubmitMode(ctx, client, opts, out)
}

func runAsyncTaskMode(ctx context.Context, client *minimax.Client, opts asyncOptions, out io.Writer) error {
	var (
		statusResp *minimax.SpeechTaskStatusResponse
		err        error
	)

	if opts.wait {
		statusResp, err = waitTask(ctx, client, opts.taskID, opts.pollInterval, out)
		if err != nil {
			return fmt.Errorf("wait async task failed: %w", err)
		}
	} else {
		statusResp, err = client.SpeechAsync.GetAsyncTask(ctx, opts.taskID)
		if err != nil {
			return fmt.Errorf("SpeechAsync.GetAsyncTask failed: %w", err)
		}
	}

	if statusResp.Status == minimax.SpeechTaskStateFailed {
		printTaskResult(out, statusResp)
		return fmt.Errorf("task_id=%s failed: %s", statusResp.TaskID, resolveTaskFailureMessage(statusResp))
	}

	printTaskResult(out, statusResp)
	return nil
}

func runAsyncSubmitMode(ctx context.Context, client *minimax.Client, opts asyncOptions, out io.Writer) error {
	submitResp, err := client.SpeechAsync.SubmitAsync(ctx, minimax.SpeechAsyncSubmitRequest{
		Model:         opts.model,
		Text:          opts.text,
		TextFileID:    opts.textFileID,
		VoiceID:       opts.voiceID,
		Speed:         opts.speed,
		Vol:           opts.volume,
		LanguageBoost: opts.languageBoost,
		OutputFormat:  opts.outputFormat,
		AudioSetting:  newSpeechAudioSetting(opts.audioFormat, opts.sampleRate, opts.bitrate, opts.channel),
	})
	if err != nil {
		return fmt.Errorf("SpeechAsync.SubmitAsync failed: %w", err)
	}

	fmt.Fprintf(out, "async submit succeeded: task_id=%s status=%s file_id=%s\n", submitResp.TaskID, displayTaskState(submitResp.Status), submitResp.FileID)

	if !opts.wait {
		fmt.Fprintln(out, "wait=false, skip polling")
		return nil
	}

	statusResp, err := waitTask(ctx, client, submitResp.TaskID, opts.pollInterval, out)
	if err != nil {
		return fmt.Errorf("wait async task failed: %w", err)
	}

	if statusResp.Status == minimax.SpeechTaskStateFailed {
		return fmt.Errorf("task_id=%s failed: %s", statusResp.TaskID, resolveTaskFailureMessage(statusResp))
	}

	printTaskResult(out, statusResp)
	return nil
}
