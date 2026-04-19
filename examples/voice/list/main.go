package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	minimax "github.com/GizClaw/minimax-go"
)

const (
	defaultBaseURL   = "https://api.minimax.io"
	defaultVoiceType = "all"
	defaultTimeout   = 30 * time.Second
)

var allowedVoiceTypes = map[string]struct{}{
	"system":           {},
	"voice_cloning":    {},
	"voice_generation": {},
	"all":              {},
}

type options struct {
	apiKey    string
	baseURL   string
	voiceType string
	pageSize  *int
	pageToken string
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
		fmt.Fprintf(os.Stderr, "voice list example failed: %v\n", err)
		os.Exit(1)
	}
}

func parseOptions() (options, error) {
	var opts options

	apiKeyDefault := os.Getenv("MINIMAX_API_KEY")
	baseURLDefault := envOrDefault("MINIMAX_BASE_URL", defaultBaseURL)
	voiceTypeDefault := envOrDefault("MINIMAX_VOICE_TYPE", defaultVoiceType)
	pageTokenDefault := os.Getenv("MINIMAX_VOICE_PAGE_TOKEN")
	pageSizeDefault, pageSizeSetByEnv, err := optionalEnvInt("MINIMAX_VOICE_PAGE_SIZE")
	if err != nil {
		return options{}, fmt.Errorf("invalid MINIMAX_VOICE_PAGE_SIZE: %w", err)
	}
	timeoutDefault := defaultTimeout
	timeoutFromEnv, timeoutSetByEnv, err := optionalEnvDuration("MINIMAX_VOICE_TIMEOUT")
	if err != nil {
		return options{}, fmt.Errorf("invalid MINIMAX_VOICE_TIMEOUT: %w", err)
	}
	if timeoutSetByEnv {
		timeoutDefault = timeoutFromEnv
	}

	pageSizeValue := pageSizeDefault

	flag.StringVar(&opts.apiKey, "api-key", apiKeyDefault, "Minimax API key (or env MINIMAX_API_KEY)")
	flag.StringVar(&opts.baseURL, "base-url", baseURLDefault, "Minimax API base URL (env: MINIMAX_BASE_URL)")
	flag.StringVar(&opts.voiceType, "voice-type", voiceTypeDefault, "Voice type filter: system/voice_cloning/voice_generation/all (env: MINIMAX_VOICE_TYPE)")
	flag.IntVar(&pageSizeValue, "page-size", pageSizeDefault, "Page size (optional, env: MINIMAX_VOICE_PAGE_SIZE)")
	flag.StringVar(&opts.pageToken, "page-token", pageTokenDefault, "Page token for next page (optional, env: MINIMAX_VOICE_PAGE_TOKEN)")
	flag.DurationVar(&opts.timeout, "timeout", timeoutDefault, "Request timeout (env: MINIMAX_VOICE_TIMEOUT, e.g. 30s)")
	flag.BoolVar(&opts.asJSON, "json", false, "Print response as formatted JSON")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: go run ./examples/voice/list [flags]\n\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nNotes:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  - API key precedence: -api-key > MINIMAX_API_KEY\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  - action_env in .bazelrc.user is not injected into go run automatically; export env manually or use -api-key\n")
	}

	flag.Parse()

	opts.apiKey = strings.TrimSpace(opts.apiKey)
	opts.baseURL = strings.TrimSpace(opts.baseURL)
	opts.pageToken = strings.TrimSpace(opts.pageToken)

	normalizedVoiceType, err := normalizeVoiceType(opts.voiceType)
	if err != nil {
		return options{}, err
	}
	opts.voiceType = normalizedVoiceType

	if opts.timeout <= 0 {
		return options{}, errors.New("timeout must be greater than 0")
	}

	if pageSizeSetByEnv || isFlagSet("page-size") {
		if pageSizeValue < 0 {
			return options{}, errors.New("page-size must be non-negative")
		}
		pageSize := pageSizeValue
		opts.pageSize = &pageSize
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

	response, err := client.Voice.ListVoices(ctx, &minimax.ListVoicesRequest{
		VoiceType: opts.voiceType,
		PageSize:  opts.pageSize,
		PageToken: opts.pageToken,
	})
	if err != nil {
		return fmt.Errorf("Voice.ListVoices failed: %w", err)
	}

	if opts.asJSON {
		payload, marshalErr := json.MarshalIndent(response, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("failed to marshal response: %w", marshalErr)
		}
		fmt.Println(string(payload))
		return nil
	}

	fmt.Printf("voices=%d has_more=%t next_page_token=%q\n", len(response.Voices), response.HasMore, response.NextPageToken)
	if len(response.Voices) == 0 {
		fmt.Println("no voices returned")
		return nil
	}

	for idx, voice := range response.Voices {
		name := strings.TrimSpace(voice.VoiceName)
		if name == "" {
			name = "-"
		}

		voiceType := strings.TrimSpace(voice.VoiceType)
		if voiceType == "" {
			voiceType = "-"
		}

		fmt.Printf("%d. id=%s type=%s name=%s\n", idx+1, voice.VoiceID, voiceType, name)
	}

	return nil
}

func envOrDefault(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return defaultValue
}

func optionalEnvInt(key string) (int, bool, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return 0, false, nil
	}

	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, true, err
	}

	return parsed, true, nil
}

func optionalEnvDuration(key string) (time.Duration, bool, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return 0, false, nil
	}

	parsed, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return 0, true, err
	}

	if parsed <= 0 {
		return 0, true, errors.New("duration must be greater than 0")
	}

	return parsed, true, nil
}

func normalizeVoiceType(raw string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		normalized = defaultVoiceType
	}

	if _, ok := allowedVoiceTypes[normalized]; !ok {
		return "", fmt.Errorf("invalid voice-type %q: must be one of system|voice_cloning|voice_generation|all", raw)
	}

	return normalized, nil
}

func isFlagSet(name string) bool {
	set := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})

	return set
}
