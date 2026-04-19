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
	httpDefaultModel   = "speech-2.6-hd"
	httpDefaultText    = "hello from minimax-go speech http example"
	httpDefaultOutFile = "speech_output.audio"
)

type httpOptions struct {
	apiKey  string
	baseURL string
	text    string
	model   string
	voiceID string
	speed   *float64
	volume  *float64
	timeout time.Duration
	output  string
}

func runHTTPCommand(args []string, stdout, stderr io.Writer) error {
	opts, err := parseHTTPOptions(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("failed to parse http flags: %w", err)
	}

	return runHTTP(opts, stdout)
}

func parseHTTPOptions(args []string, out io.Writer) (httpOptions, error) {
	var opts httpOptions

	apiKeyDefault := os.Getenv("MINIMAX_API_KEY")
	baseURLDefault := envOrDefault("MINIMAX_BASE_URL", exampleDefaultBaseURL)
	textDefault := envOrDefault("MINIMAX_SPEECH_TEXT", httpDefaultText)
	modelDefault := envOrDefault("MINIMAX_SPEECH_MODEL", httpDefaultModel)
	voiceDefault := os.Getenv("MINIMAX_SPEECH_VOICE_ID")
	speedDefault, speedSetByEnv, err := optionalEnvFloat64("MINIMAX_SPEECH_SPEED")
	if err != nil {
		return httpOptions{}, fmt.Errorf("invalid MINIMAX_SPEECH_SPEED: %w", err)
	}
	if !speedSetByEnv {
		speedDefault = 1
	}

	volumeDefault, volumeSetByEnv, err := optionalEnvFloat64("MINIMAX_SPEECH_VOLUME")
	if err != nil {
		return httpOptions{}, fmt.Errorf("invalid MINIMAX_SPEECH_VOLUME: %w", err)
	}
	if !volumeSetByEnv {
		volumeDefault = 1
	}

	timeoutDefault := envDurationOrDefault("MINIMAX_SPEECH_TIMEOUT", 30*time.Second)
	outputDefault := envOrDefault("MINIMAX_SPEECH_OUTPUT", httpDefaultOutFile)

	fs := flag.NewFlagSet("http", flag.ContinueOnError)
	fs.SetOutput(out)

	speedValue := speedDefault
	volumeValue := volumeDefault

	fs.StringVar(&opts.apiKey, "api-key", apiKeyDefault, "Minimax API key (or env MINIMAX_API_KEY)")
	fs.StringVar(&opts.baseURL, "base-url", baseURLDefault, "Minimax API base URL (env: MINIMAX_BASE_URL)")
	fs.StringVar(&opts.text, "text", textDefault, "Text to synthesize (env: MINIMAX_SPEECH_TEXT)")
	fs.StringVar(&opts.model, "model", modelDefault, "Model name (optional, env: MINIMAX_SPEECH_MODEL)")
	fs.StringVar(&opts.voiceID, "voice-id", voiceDefault, "Voice ID (optional, env: MINIMAX_SPEECH_VOICE_ID)")
	fs.Float64Var(&speedValue, "speed", speedDefault, "Speech speed (optional, env: MINIMAX_SPEECH_SPEED)")
	fs.Float64Var(&volumeValue, "volume", volumeDefault, "Speech volume (optional, env: MINIMAX_SPEECH_VOLUME)")
	fs.DurationVar(&opts.timeout, "timeout", timeoutDefault, "Request timeout (env: MINIMAX_SPEECH_TIMEOUT, e.g. 30s)")
	fs.StringVar(&opts.output, "output", outputDefault, "Output audio file path (env: MINIMAX_SPEECH_OUTPUT)")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: go run ./examples/speech http [flags]\n\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nNotes:\n")
		fmt.Fprintf(fs.Output(), "  - API key precedence: -api-key > MINIMAX_API_KEY\n")
	}

	if err := fs.Parse(args); err != nil {
		return httpOptions{}, err
	}

	opts.apiKey = strings.TrimSpace(opts.apiKey)
	opts.baseURL = strings.TrimSpace(opts.baseURL)
	opts.text = strings.TrimSpace(opts.text)
	opts.model = strings.TrimSpace(opts.model)
	opts.voiceID = strings.TrimSpace(opts.voiceID)
	opts.output = strings.TrimSpace(opts.output)

	if opts.timeout <= 0 {
		return httpOptions{}, errors.New("timeout must be greater than 0")
	}

	if speedSetByEnv || flagWasSet(fs, "speed") {
		speed := speedValue
		opts.speed = &speed
	}

	if volumeSetByEnv || flagWasSet(fs, "volume") {
		volume := volumeValue
		opts.volume = &volume
	}

	return opts, nil
}

func runHTTP(opts httpOptions, out io.Writer) error {
	if opts.apiKey == "" {
		return errors.New("missing API key: use -api-key or set MINIMAX_API_KEY")
	}

	if opts.baseURL == "" {
		return errors.New("base-url cannot be empty")
	}

	if opts.text == "" {
		return errors.New("text cannot be empty")
	}

	if opts.output == "" {
		return errors.New("output cannot be empty")
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

	response, err := client.Speech.Synthesize(ctx, minimax.SpeechRequest{
		Model:   opts.model,
		Text:    opts.text,
		VoiceID: opts.voiceID,
		Speed:   opts.speed,
		Vol:     opts.volume,
	})
	if err != nil {
		return fmt.Errorf("Speech.Synthesize failed: %w", err)
	}

	if len(response.Audio) == 0 {
		return errors.New("synthesis succeeded but returned empty audio bytes")
	}

	if err := ensureOutputDir(opts.output); err != nil {
		return err
	}

	if err := os.WriteFile(opts.output, response.Audio, 0o644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Fprintf(out, "http synthesis succeeded, wrote %d bytes to %s\n", len(response.Audio), opts.output)
	return nil
}
