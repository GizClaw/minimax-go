package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	minimax "github.com/GizClaw/minimax-go"
)

const (
	defaultBaseURL = "https://api.minimax.io"
	defaultTimeout = 30 * time.Second
)

type options struct {
	apiKey      string
	baseURL     string
	prompt      string
	previewText string
	voiceID     string
	timeout     time.Duration
	asJSON      bool
}

func main() {
	opts, err := parseOptions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse flags: %v\n", err)
		os.Exit(2)
	}

	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "voice design example failed: %v\n", err)
		os.Exit(1)
	}
}

func parseOptions() (options, error) {
	var opts options

	apiKeyDefault := os.Getenv("MINIMAX_API_KEY")
	baseURLDefault := envOrDefault("MINIMAX_BASE_URL", defaultBaseURL)
	promptDefault := os.Getenv("MINIMAX_VOICE_DESIGN_PROMPT")
	previewTextDefault := os.Getenv("MINIMAX_VOICE_DESIGN_PREVIEW_TEXT")
	voiceIDDefault := os.Getenv("MINIMAX_VOICE_DESIGN_VOICE_ID")
	timeoutDefault := envDurationOrDefault("MINIMAX_VOICE_DESIGN_TIMEOUT", defaultTimeout)

	flag.StringVar(&opts.apiKey, "api-key", apiKeyDefault, "Minimax API key (or env MINIMAX_API_KEY)")
	flag.StringVar(&opts.baseURL, "base-url", baseURLDefault, "Minimax API base URL (env: MINIMAX_BASE_URL)")
	flag.StringVar(&opts.prompt, "prompt", promptDefault, "Voice description prompt (env: MINIMAX_VOICE_DESIGN_PROMPT)")
	flag.StringVar(&opts.previewText, "preview-text", previewTextDefault, "Preview text (env: MINIMAX_VOICE_DESIGN_PREVIEW_TEXT)")
	flag.StringVar(&opts.voiceID, "voice-id", voiceIDDefault, "Optional custom voice_id (env: MINIMAX_VOICE_DESIGN_VOICE_ID)")
	flag.DurationVar(&opts.timeout, "timeout", timeoutDefault, "Request timeout (env: MINIMAX_VOICE_DESIGN_TIMEOUT, e.g. 30s)")
	flag.BoolVar(&opts.asJSON, "json", false, "Print response as formatted JSON")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: go run ./examples/voice/design [flags]\n\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nNotes:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  - API key precedence: -api-key > MINIMAX_API_KEY\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  - required flags: -prompt and -preview-text\n")
	}

	flag.Parse()

	opts.apiKey = strings.TrimSpace(opts.apiKey)
	opts.baseURL = strings.TrimSpace(opts.baseURL)
	opts.prompt = strings.TrimSpace(opts.prompt)
	opts.previewText = strings.TrimSpace(opts.previewText)
	opts.voiceID = strings.TrimSpace(opts.voiceID)

	if opts.timeout <= 0 {
		return options{}, errors.New("timeout must be greater than 0")
	}

	if opts.prompt == "" {
		return options{}, errors.New("prompt cannot be empty")
	}

	if opts.previewText == "" {
		return options{}, errors.New("preview-text cannot be empty")
	}

	return opts, nil
}

func run(opts options) error {
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

	response, err := client.Voice.DesignVoice(ctx, &minimax.DesignVoiceRequest{
		Prompt:      opts.prompt,
		PreviewText: opts.previewText,
		VoiceID:     opts.voiceID,
	})
	if err != nil {
		return fmt.Errorf("Voice.DesignVoice failed: %w", err)
	}

	if opts.asJSON {
		payload, marshalErr := json.MarshalIndent(response, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("failed to marshal response: %w", marshalErr)
		}
		fmt.Println(string(payload))
		return nil
	}

	fmt.Println("design succeeded")
	fmt.Printf("  voice_id: %s\n", response.VoiceID)
	fmt.Printf("  trial_audio(hex): %s\n", response.TrialAudio)

	return nil
}

func envOrDefault(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return defaultValue
}

func envDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return defaultValue
	}

	parsed, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return defaultValue
	}

	if parsed <= 0 {
		return defaultValue
	}

	return parsed
}
