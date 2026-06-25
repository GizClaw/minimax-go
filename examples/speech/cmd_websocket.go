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
	webSocketDefaultModel   = "speech-2.8-turbo"
	webSocketDefaultText    = "hello from minimax-go speech websocket example"
	webSocketDefaultOutFile = "speech_websocket_output.audio"
)

type webSocketOptions struct {
	apiKey        string
	baseURL       string
	text          string
	model         string
	voiceID       string
	speed         *float64
	volume        *float64
	languageBoost string
	audioFormat   string
	sampleRate    *int
	bitrate       *int
	channel       *int
	timeout       time.Duration
	output        string
}

func runWebSocketCommand(args []string, stdout, stderr io.Writer) error {
	opts, err := parseWebSocketOptions(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("failed to parse websocket flags: %w", err)
	}

	return runWebSocket(opts, stdout)
}

func parseWebSocketOptions(args []string, out io.Writer) (webSocketOptions, error) {
	var opts webSocketOptions

	apiKeyDefault := os.Getenv("MINIMAX_API_KEY")
	baseURLDefault := envOrDefault("MINIMAX_BASE_URL", exampleDefaultBaseURL)
	textDefault := envOrDefaultFromKeys([]string{"MINIMAX_SPEECH_WEBSOCKET_TEXT", "MINIMAX_SPEECH_TEXT"}, webSocketDefaultText)
	modelDefault := envOrDefaultFromKeys([]string{"MINIMAX_SPEECH_WEBSOCKET_MODEL", "MINIMAX_SPEECH_MODEL"}, webSocketDefaultModel)
	voiceDefault := envOrDefaultFromKeys([]string{"MINIMAX_SPEECH_WEBSOCKET_VOICE_ID", "MINIMAX_SPEECH_VOICE_ID"}, "")

	speedDefault, speedSetByEnv, err := optionalEnvFloat64FromKeys("MINIMAX_SPEECH_WEBSOCKET_SPEED", "MINIMAX_SPEECH_SPEED")
	if err != nil {
		return webSocketOptions{}, fmt.Errorf("invalid speech speed env: %w", err)
	}
	if !speedSetByEnv {
		speedDefault = 1
	}

	volumeDefault, volumeSetByEnv, err := optionalEnvFloat64FromKeys("MINIMAX_SPEECH_WEBSOCKET_VOLUME", "MINIMAX_SPEECH_VOLUME")
	if err != nil {
		return webSocketOptions{}, fmt.Errorf("invalid speech volume env: %w", err)
	}
	if !volumeSetByEnv {
		volumeDefault = 1
	}

	timeoutDefault := envDurationOrDefaultFromKeys([]string{"MINIMAX_SPEECH_WEBSOCKET_TIMEOUT", "MINIMAX_SPEECH_TIMEOUT"}, 30*time.Second)
	outputDefault := envOrDefaultFromKeys([]string{"MINIMAX_SPEECH_WEBSOCKET_OUTPUT", "MINIMAX_SPEECH_OUTPUT"}, webSocketDefaultOutFile)
	languageBoostDefault := envOrDefaultFromKeys([]string{"MINIMAX_SPEECH_WEBSOCKET_LANGUAGE_BOOST", "MINIMAX_SPEECH_LANGUAGE_BOOST"}, "")
	audioFormatDefault := envOrDefaultFromKeys([]string{"MINIMAX_SPEECH_WEBSOCKET_AUDIO_FORMAT", "MINIMAX_SPEECH_AUDIO_FORMAT"}, "")
	sampleRateDefault, sampleRateSetByEnv, err := optionalEnvIntFromKeys("MINIMAX_SPEECH_WEBSOCKET_SAMPLE_RATE", "MINIMAX_SPEECH_SAMPLE_RATE")
	if err != nil {
		return webSocketOptions{}, fmt.Errorf("invalid speech sample rate env: %w", err)
	}
	bitrateDefault, bitrateSetByEnv, err := optionalEnvIntFromKeys("MINIMAX_SPEECH_WEBSOCKET_BITRATE", "MINIMAX_SPEECH_BITRATE")
	if err != nil {
		return webSocketOptions{}, fmt.Errorf("invalid speech bitrate env: %w", err)
	}
	channelDefault, channelSetByEnv, err := optionalEnvIntFromKeys("MINIMAX_SPEECH_WEBSOCKET_CHANNEL", "MINIMAX_SPEECH_CHANNEL")
	if err != nil {
		return webSocketOptions{}, fmt.Errorf("invalid speech channel env: %w", err)
	}

	fs := flag.NewFlagSet("websocket", flag.ContinueOnError)
	fs.SetOutput(out)

	speedValue := speedDefault
	volumeValue := volumeDefault
	sampleRateValue := sampleRateDefault
	bitrateValue := bitrateDefault
	channelValue := channelDefault

	fs.StringVar(&opts.apiKey, "api-key", apiKeyDefault, "Minimax API key (or env MINIMAX_API_KEY)")
	fs.StringVar(&opts.baseURL, "base-url", baseURLDefault, "Minimax API base URL (env: MINIMAX_BASE_URL)")
	fs.StringVar(&opts.text, "text", textDefault, "Text to synthesize (env: MINIMAX_SPEECH_WEBSOCKET_TEXT)")
	fs.StringVar(&opts.model, "model", modelDefault, "Model name (env: MINIMAX_SPEECH_WEBSOCKET_MODEL)")
	fs.StringVar(&opts.voiceID, "voice-id", voiceDefault, "Voice ID (env: MINIMAX_SPEECH_WEBSOCKET_VOICE_ID)")
	fs.Float64Var(&speedValue, "speed", speedDefault, "Speech speed (env: MINIMAX_SPEECH_WEBSOCKET_SPEED)")
	fs.Float64Var(&volumeValue, "volume", volumeDefault, "Speech volume (env: MINIMAX_SPEECH_WEBSOCKET_VOLUME)")
	fs.StringVar(&opts.languageBoost, "language-boost", languageBoostDefault, "Language boost value, e.g. English or auto (env: MINIMAX_SPEECH_WEBSOCKET_LANGUAGE_BOOST)")
	fs.StringVar(&opts.audioFormat, "audio-format", audioFormatDefault, "Audio format, e.g. mp3/wav/flac (env: MINIMAX_SPEECH_WEBSOCKET_AUDIO_FORMAT)")
	fs.IntVar(&sampleRateValue, "sample-rate", sampleRateDefault, "Audio sample rate (env: MINIMAX_SPEECH_WEBSOCKET_SAMPLE_RATE)")
	fs.IntVar(&bitrateValue, "bitrate", bitrateDefault, "Audio bitrate (env: MINIMAX_SPEECH_WEBSOCKET_BITRATE)")
	fs.IntVar(&channelValue, "channel", channelDefault, "Audio channel count (env: MINIMAX_SPEECH_WEBSOCKET_CHANNEL)")
	fs.DurationVar(&opts.timeout, "timeout", timeoutDefault, "Request timeout (env: MINIMAX_SPEECH_WEBSOCKET_TIMEOUT, e.g. 30s)")
	fs.StringVar(&opts.output, "output", outputDefault, "Output audio file path (env: MINIMAX_SPEECH_WEBSOCKET_OUTPUT)")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: go run ./examples/speech websocket [flags]\n\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return webSocketOptions{}, err
	}

	opts.apiKey = strings.TrimSpace(opts.apiKey)
	opts.baseURL = strings.TrimSpace(opts.baseURL)
	opts.text = strings.TrimSpace(opts.text)
	opts.model = strings.TrimSpace(opts.model)
	opts.voiceID = strings.TrimSpace(opts.voiceID)
	opts.languageBoost = strings.TrimSpace(opts.languageBoost)
	opts.audioFormat = strings.TrimSpace(opts.audioFormat)
	opts.output = strings.TrimSpace(opts.output)

	if opts.timeout <= 0 {
		return webSocketOptions{}, errors.New("timeout must be greater than 0")
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

func runWebSocket(opts webSocketOptions, out io.Writer) (retErr error) {
	if opts.apiKey == "" {
		return errors.New("missing API key: use -api-key or set MINIMAX_API_KEY")
	}
	if opts.baseURL == "" {
		return errors.New("base-url cannot be empty")
	}
	if opts.text == "" {
		return errors.New("text cannot be empty")
	}
	if opts.voiceID == "" {
		return errors.New("voice-id cannot be empty")
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

	ws, err := client.Speech.OpenWebSocket(ctx, minimax.SpeechWebSocketRequest{
		Model:         opts.model,
		Text:          opts.text,
		VoiceID:       opts.voiceID,
		Speed:         opts.speed,
		Vol:           opts.volume,
		LanguageBoost: opts.languageBoost,
		AudioSetting:  newSpeechAudioSetting(opts.audioFormat, opts.sampleRate, opts.bitrate, opts.channel),
	})
	if err != nil {
		return fmt.Errorf("Speech.OpenWebSocket failed: %w", err)
	}
	defer closeWithRetError(ws, "speech websocket", &retErr)

	if err := ensureOutputDir(opts.output); err != nil {
		return err
	}

	outputFile, err := os.OpenFile(opts.output, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}
	defer closeWithRetError(outputFile, "output file", &retErr)

	totalBytes, chunkCount, err := writeSpeechWebSocketToFile(ctx, ws, outputFile)
	if err != nil {
		return err
	}
	if err := outputFile.Sync(); err != nil {
		return fmt.Errorf("failed to flush output file: %w", err)
	}
	if totalBytes == 0 {
		return errors.New("websocket synthesis finished but no audio chunk was received")
	}

	fmt.Fprintf(out, "websocket synthesis succeeded, wrote %d bytes from %d chunks to %s\n", totalBytes, chunkCount, opts.output)
	return nil
}

func writeSpeechWebSocketToFile(ctx context.Context, ws *minimax.SpeechWebSocket, outputFile *os.File) (totalBytes int, chunkCount int, err error) {
	for {
		event, nextErr := ws.Next(ctx)
		if errors.Is(nextErr, io.EOF) {
			return totalBytes, chunkCount, nil
		}
		if nextErr != nil {
			return 0, 0, fmt.Errorf("failed to read websocket event: %w", nextErr)
		}
		if event == nil {
			continue
		}
		if len(event.Audio) > 0 {
			if _, writeErr := outputFile.Write(event.Audio); writeErr != nil {
				return 0, 0, fmt.Errorf("failed to write audio chunk: %w", writeErr)
			}
			totalBytes += len(event.Audio)
			chunkCount++
		}
		if event.Done {
			return totalBytes, chunkCount, nil
		}
	}
}
