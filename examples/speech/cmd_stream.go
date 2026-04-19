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
	streamDefaultModel   = "speech-2.6-hd"
	streamDefaultText    = "hello from minimax-go speech stream example"
	streamDefaultOutFile = "speech_stream_output.audio"
)

type streamOptions struct {
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

func runStreamCommand(args []string, stdout, stderr io.Writer) error {
	opts, err := parseStreamOptions(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("failed to parse stream flags: %w", err)
	}

	return runStream(opts, stdout)
}

func parseStreamOptions(args []string, out io.Writer) (streamOptions, error) {
	var opts streamOptions

	apiKeyDefault := os.Getenv("MINIMAX_API_KEY")
	baseURLDefault := envOrDefault("MINIMAX_BASE_URL", exampleDefaultBaseURL)
	textDefault := envOrDefaultFromKeys([]string{"MINIMAX_SPEECH_STREAM_TEXT", "MINIMAX_SPEECH_TEXT"}, streamDefaultText)
	modelDefault := envOrDefaultFromKeys([]string{"MINIMAX_SPEECH_STREAM_MODEL", "MINIMAX_SPEECH_MODEL"}, streamDefaultModel)
	voiceDefault := envOrDefaultFromKeys([]string{"MINIMAX_SPEECH_STREAM_VOICE_ID", "MINIMAX_SPEECH_VOICE_ID"}, "")

	speedDefault, speedSetByEnv, err := optionalEnvFloat64FromKeys("MINIMAX_SPEECH_STREAM_SPEED", "MINIMAX_SPEECH_SPEED")
	if err != nil {
		return streamOptions{}, fmt.Errorf("invalid speech speed env: %w", err)
	}
	if !speedSetByEnv {
		speedDefault = 1
	}

	volumeDefault, volumeSetByEnv, err := optionalEnvFloat64FromKeys("MINIMAX_SPEECH_STREAM_VOLUME", "MINIMAX_SPEECH_VOLUME")
	if err != nil {
		return streamOptions{}, fmt.Errorf("invalid speech volume env: %w", err)
	}
	if !volumeSetByEnv {
		volumeDefault = 1
	}

	timeoutDefault := envDurationOrDefaultFromKeys([]string{"MINIMAX_SPEECH_STREAM_TIMEOUT", "MINIMAX_SPEECH_TIMEOUT"}, 30*time.Second)
	outputDefault := envOrDefaultFromKeys([]string{"MINIMAX_SPEECH_STREAM_OUTPUT", "MINIMAX_SPEECH_OUTPUT"}, streamDefaultOutFile)

	fs := flag.NewFlagSet("stream", flag.ContinueOnError)
	fs.SetOutput(out)

	speedValue := speedDefault
	volumeValue := volumeDefault

	fs.StringVar(&opts.apiKey, "api-key", apiKeyDefault, "Minimax API key (or env MINIMAX_API_KEY)")
	fs.StringVar(&opts.baseURL, "base-url", baseURLDefault, "Minimax API base URL (env: MINIMAX_BASE_URL)")
	fs.StringVar(&opts.text, "text", textDefault, "Text to synthesize (env: MINIMAX_SPEECH_STREAM_TEXT)")
	fs.StringVar(&opts.model, "model", modelDefault, "Model name (optional, env: MINIMAX_SPEECH_STREAM_MODEL)")
	fs.StringVar(&opts.voiceID, "voice-id", voiceDefault, "Voice ID (optional, env: MINIMAX_SPEECH_STREAM_VOICE_ID)")
	fs.Float64Var(&speedValue, "speed", speedDefault, "Speech speed (optional, env: MINIMAX_SPEECH_STREAM_SPEED)")
	fs.Float64Var(&volumeValue, "volume", volumeDefault, "Speech volume (optional, env: MINIMAX_SPEECH_STREAM_VOLUME)")
	fs.DurationVar(&opts.timeout, "timeout", timeoutDefault, "Request timeout (env: MINIMAX_SPEECH_STREAM_TIMEOUT, e.g. 30s)")
	fs.StringVar(&opts.output, "output", outputDefault, "Output audio file path (env: MINIMAX_SPEECH_STREAM_OUTPUT)")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: go run ./examples/speech stream [flags]\n\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return streamOptions{}, err
	}

	opts.apiKey = strings.TrimSpace(opts.apiKey)
	opts.baseURL = strings.TrimSpace(opts.baseURL)
	opts.text = strings.TrimSpace(opts.text)
	opts.model = strings.TrimSpace(opts.model)
	opts.voiceID = strings.TrimSpace(opts.voiceID)
	opts.output = strings.TrimSpace(opts.output)

	if opts.timeout <= 0 {
		return streamOptions{}, errors.New("timeout must be greater than 0")
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

func runStream(opts streamOptions, out io.Writer) (retErr error) {
	if err := validateStreamRuntimeOptions(opts); err != nil {
		return err
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

	speechStream, err := client.Speech.OpenStream(ctx, minimax.SpeechStreamRequest{
		Model:   opts.model,
		Text:    opts.text,
		VoiceID: opts.voiceID,
		Speed:   opts.speed,
		Vol:     opts.volume,
	})
	if err != nil {
		return fmt.Errorf("Speech.OpenStream failed: %w", err)
	}
	defer closeWithRetError(speechStream, "speech stream", &retErr)

	if err := ensureOutputDir(opts.output); err != nil {
		return err
	}

	outputFile, err := os.OpenFile(opts.output, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}
	defer closeWithRetError(outputFile, "output file", &retErr)

	totalBytes, chunkCount, err := writeSpeechStreamToFile(speechStream, outputFile)
	if err != nil {
		return err
	}

	if err := outputFile.Sync(); err != nil {
		return fmt.Errorf("failed to flush output file: %w", err)
	}

	if totalBytes == 0 {
		return errors.New("stream synthesis finished but no audio chunk was received")
	}

	fmt.Fprintf(out, "stream synthesis succeeded, wrote %d bytes from %d chunks to %s\n", totalBytes, chunkCount, opts.output)
	return nil
}

func validateStreamRuntimeOptions(opts streamOptions) error {
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

	return nil
}

func writeSpeechStreamToFile(speechStream *minimax.SpeechStream, outputFile *os.File) (totalBytes int, chunkCount int, err error) {
	for {
		chunk, nextErr := speechStream.Next()
		if errors.Is(nextErr, io.EOF) {
			return totalBytes, chunkCount, nil
		}
		if nextErr != nil {
			return 0, 0, fmt.Errorf("failed to read stream chunk: %w", nextErr)
		}
		if chunk == nil {
			continue
		}

		if len(chunk.Audio) > 0 {
			if _, writeErr := outputFile.Write(chunk.Audio); writeErr != nil {
				return 0, 0, fmt.Errorf("failed to write audio chunk: %w", writeErr)
			}
			totalBytes += len(chunk.Audio)
			chunkCount++
		}

		if chunk.Done {
			return totalBytes, chunkCount, nil
		}
	}
}

func closeWithRetError(closer io.Closer, name string, retErr *error) {
	if closer == nil {
		return
	}
	if err := closer.Close(); err != nil && retErr != nil && *retErr == nil {
		*retErr = fmt.Errorf("failed to close %s: %w", name, err)
	}
}
