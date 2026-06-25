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
	apiKey        string
	baseURL       string
	text          string
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
	output        string
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
	languageBoostDefault := os.Getenv("MINIMAX_SPEECH_LANGUAGE_BOOST")
	outputFormatDefault := envOrDefault("MINIMAX_SPEECH_OUTPUT_FORMAT", "hex")
	audioFormatDefault := os.Getenv("MINIMAX_SPEECH_AUDIO_FORMAT")
	sampleRateDefault, sampleRateSetByEnv, err := optionalEnvIntFromKeys("MINIMAX_SPEECH_SAMPLE_RATE")
	if err != nil {
		return httpOptions{}, fmt.Errorf("invalid speech sample rate env: %w", err)
	}
	bitrateDefault, bitrateSetByEnv, err := optionalEnvIntFromKeys("MINIMAX_SPEECH_BITRATE")
	if err != nil {
		return httpOptions{}, fmt.Errorf("invalid speech bitrate env: %w", err)
	}
	channelDefault, channelSetByEnv, err := optionalEnvIntFromKeys("MINIMAX_SPEECH_CHANNEL")
	if err != nil {
		return httpOptions{}, fmt.Errorf("invalid speech channel env: %w", err)
	}

	fs := flag.NewFlagSet("http", flag.ContinueOnError)
	fs.SetOutput(out)

	speedValue := speedDefault
	volumeValue := volumeDefault
	sampleRateValue := sampleRateDefault
	bitrateValue := bitrateDefault
	channelValue := channelDefault

	fs.StringVar(&opts.apiKey, "api-key", apiKeyDefault, "Minimax API key (or env MINIMAX_API_KEY)")
	fs.StringVar(&opts.baseURL, "base-url", baseURLDefault, "Minimax API base URL (env: MINIMAX_BASE_URL)")
	fs.StringVar(&opts.text, "text", textDefault, "Text to synthesize (env: MINIMAX_SPEECH_TEXT)")
	fs.StringVar(&opts.model, "model", modelDefault, "Model name (optional, env: MINIMAX_SPEECH_MODEL)")
	fs.StringVar(&opts.voiceID, "voice-id", voiceDefault, "Voice ID (optional, env: MINIMAX_SPEECH_VOICE_ID)")
	fs.Float64Var(&speedValue, "speed", speedDefault, "Speech speed (optional, env: MINIMAX_SPEECH_SPEED)")
	fs.Float64Var(&volumeValue, "volume", volumeDefault, "Speech volume (optional, env: MINIMAX_SPEECH_VOLUME)")
	fs.StringVar(&opts.languageBoost, "language-boost", languageBoostDefault, "Language boost value, e.g. English or auto (env: MINIMAX_SPEECH_LANGUAGE_BOOST)")
	fs.StringVar(&opts.outputFormat, "output-format", outputFormatDefault, "Output format: hex or url (env: MINIMAX_SPEECH_OUTPUT_FORMAT)")
	fs.StringVar(&opts.audioFormat, "audio-format", audioFormatDefault, "Audio format, e.g. mp3/wav/flac (env: MINIMAX_SPEECH_AUDIO_FORMAT)")
	fs.IntVar(&sampleRateValue, "sample-rate", sampleRateDefault, "Audio sample rate (env: MINIMAX_SPEECH_SAMPLE_RATE)")
	fs.IntVar(&bitrateValue, "bitrate", bitrateDefault, "Audio bitrate (env: MINIMAX_SPEECH_BITRATE)")
	fs.IntVar(&channelValue, "channel", channelDefault, "Audio channel count (env: MINIMAX_SPEECH_CHANNEL)")
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
	opts.languageBoost = strings.TrimSpace(opts.languageBoost)
	opts.outputFormat = strings.TrimSpace(opts.outputFormat)
	opts.audioFormat = strings.TrimSpace(opts.audioFormat)
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
	if sampleRateSetByEnv || flagWasSet(fs, "sample-rate") {
		sampleRate := sampleRateValue
		opts.sampleRate = &sampleRate
	}
	if bitrateSetByEnv || flagWasSet(fs, "bitrate") {
		bitrate := bitrateValue
		opts.bitrate = &bitrate
	}
	if channelSetByEnv || flagWasSet(fs, "channel") {
		channel := channelValue
		opts.channel = &channel
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
		Model:         opts.model,
		Text:          opts.text,
		VoiceID:       opts.voiceID,
		Speed:         opts.speed,
		Vol:           opts.volume,
		OutputFormat:  opts.outputFormat,
		LanguageBoost: opts.languageBoost,
		AudioSetting:  newSpeechAudioSetting(opts.audioFormat, opts.sampleRate, opts.bitrate, opts.channel),
	})
	if err != nil {
		return fmt.Errorf("Speech.Synthesize failed: %w", err)
	}

	if response.AudioURL != "" {
		fmt.Fprintf(out, "http synthesis succeeded, audio_url=%s\n", response.AudioURL)
		return nil
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
