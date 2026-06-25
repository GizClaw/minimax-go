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
	apiKey    string
	baseURL   string
	voiceID   string
	voiceType string
	timeout   time.Duration
	asJSON    bool
}

func main() {
	opts, err := parseOptions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse flags: %v\n", err)
		os.Exit(2)
	}

	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "voice delete example failed: %v\n", err)
		os.Exit(1)
	}
}

func parseOptions() (options, error) {
	var opts options

	flag.StringVar(&opts.apiKey, "api-key", os.Getenv("MINIMAX_API_KEY"), "Minimax API key (or env MINIMAX_API_KEY)")
	flag.StringVar(&opts.baseURL, "base-url", envOrDefault("MINIMAX_BASE_URL", defaultBaseURL), "Minimax API base URL (env: MINIMAX_BASE_URL)")
	flag.StringVar(&opts.voiceID, "voice-id", os.Getenv("MINIMAX_VOICE_DELETE_VOICE_ID"), "Voice ID to delete (env: MINIMAX_VOICE_DELETE_VOICE_ID)")
	flag.StringVar(&opts.voiceType, "voice-type", envOrDefault("MINIMAX_VOICE_DELETE_VOICE_TYPE", minimax.VoiceDeleteTypeGeneration), "Voice type: voice_generation or voice_cloning (env: MINIMAX_VOICE_DELETE_VOICE_TYPE)")
	flag.DurationVar(&opts.timeout, "timeout", envDurationOrDefault("MINIMAX_VOICE_DELETE_TIMEOUT", defaultTimeout), "Request timeout (env: MINIMAX_VOICE_DELETE_TIMEOUT, e.g. 30s)")
	flag.BoolVar(&opts.asJSON, "json", false, "Print response as formatted JSON")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: go run ./examples/voice/delete -voice-id <owned_voice_id> [flags]\n\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	opts.apiKey = strings.TrimSpace(opts.apiKey)
	opts.baseURL = strings.TrimSpace(opts.baseURL)
	opts.voiceID = strings.TrimSpace(opts.voiceID)
	opts.voiceType = strings.TrimSpace(opts.voiceType)
	if opts.timeout <= 0 {
		return options{}, errors.New("timeout must be greater than 0")
	}
	if opts.voiceID == "" {
		return options{}, errors.New("voice-id cannot be empty")
	}
	if opts.voiceType != minimax.VoiceDeleteTypeGeneration && opts.voiceType != minimax.VoiceDeleteTypeCloning {
		return options{}, fmt.Errorf("voice-type must be %q or %q", minimax.VoiceDeleteTypeGeneration, minimax.VoiceDeleteTypeCloning)
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

	response, err := client.Voice.DeleteVoice(ctx, minimax.DeleteVoiceRequest{
		VoiceID:   opts.voiceID,
		VoiceType: opts.voiceType,
	})
	if err != nil {
		return fmt.Errorf("Voice.DeleteVoice failed: %w", err)
	}

	if opts.asJSON {
		payload, marshalErr := json.MarshalIndent(response, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("failed to marshal response: %w", marshalErr)
		}
		fmt.Println(string(payload))
		return nil
	}

	fmt.Println("delete succeeded")
	fmt.Printf("  voice_id: %s\n", response.VoiceID)
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
	if err != nil || parsed <= 0 {
		return defaultValue
	}
	return parsed
}
