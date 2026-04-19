package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	minimax "github.com/GizClaw/minimax-go"
)

const (
	defaultBaseURL       = "https://api.minimax.io"
	defaultTimeout       = 30 * time.Second
	defaultUploadPurpose = "voice_clone"
)

type options struct {
	apiKey       string
	baseURL      string
	cloneVoiceID string
	audioURL     string
	fileID       string
	inputPath    string
	fileName     string
	contentType  string
	purpose      string
	timeout      time.Duration
	asJSON       bool
}

func main() {
	opts, err := parseOptions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse flags: %v\n", err)
		os.Exit(2)
	}

	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "voice clone example failed: %v\n", err)
		os.Exit(1)
	}
}

func parseOptions() (options, error) {
	var opts options

	apiKeyDefault := os.Getenv("MINIMAX_API_KEY")
	baseURLDefault := envOrDefault("MINIMAX_BASE_URL", defaultBaseURL)
	cloneVoiceIDDefault := os.Getenv("MINIMAX_VOICE_CLONE_VOICE_ID")
	audioURLDefault := os.Getenv("MINIMAX_VOICE_CLONE_AUDIO_URL")
	fileIDDefault := os.Getenv("MINIMAX_VOICE_CLONE_FILE_ID")
	inputPathDefault := os.Getenv("MINIMAX_VOICE_CLONE_FILE_INPUT")
	fileNameDefault := os.Getenv("MINIMAX_VOICE_CLONE_FILE_NAME")
	contentTypeDefault := os.Getenv("MINIMAX_VOICE_CLONE_CONTENT_TYPE")
	purposeDefault := envOrDefault("MINIMAX_VOICE_CLONE_PURPOSE", defaultUploadPurpose)
	timeoutDefault := envDurationOrDefault("MINIMAX_VOICE_CLONE_TIMEOUT", defaultTimeout)

	flag.StringVar(&opts.apiKey, "api-key", apiKeyDefault, "Minimax API key (or env MINIMAX_API_KEY)")
	flag.StringVar(&opts.baseURL, "base-url", baseURLDefault, "Minimax API base URL (env: MINIMAX_BASE_URL)")
	flag.StringVar(&opts.cloneVoiceID, "voice-id", cloneVoiceIDDefault, "Target cloned voice_id (env: MINIMAX_VOICE_CLONE_VOICE_ID)")
	flag.StringVar(&opts.audioURL, "audio-url", audioURLDefault, "Source audio URL (env: MINIMAX_VOICE_CLONE_AUDIO_URL)")
	flag.StringVar(&opts.fileID, "file-id", fileIDDefault, "Uploaded file_id (env: MINIMAX_VOICE_CLONE_FILE_ID)")
	flag.StringVar(&opts.inputPath, "input", inputPathDefault, "Local audio path; upload first then clone by returned file_id (env: MINIMAX_VOICE_CLONE_FILE_INPUT)")
	flag.StringVar(&opts.fileName, "file-name", fileNameDefault, "Uploaded file name override for -input (env: MINIMAX_VOICE_CLONE_FILE_NAME)")
	flag.StringVar(&opts.contentType, "content-type", contentTypeDefault, "MIME type override for -input upload (env: MINIMAX_VOICE_CLONE_CONTENT_TYPE)")
	flag.StringVar(&opts.purpose, "purpose", purposeDefault, "Upload purpose for -input flow (env: MINIMAX_VOICE_CLONE_PURPOSE)")
	flag.DurationVar(&opts.timeout, "timeout", timeoutDefault, "Request timeout (env: MINIMAX_VOICE_CLONE_TIMEOUT, e.g. 30s)")
	flag.BoolVar(&opts.asJSON, "json", false, "Print response as formatted JSON")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: go run ./examples/voice/clone [flags]\n\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nNotes:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  - API key precedence: -api-key > MINIMAX_API_KEY\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  - required: -voice-id and one source among -audio-url / -file-id / -input\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  - in China region, audio-url may be unsupported; use -input or -file-id\n")
	}

	flag.Parse()

	opts.apiKey = strings.TrimSpace(opts.apiKey)
	opts.baseURL = strings.TrimSpace(opts.baseURL)
	opts.cloneVoiceID = strings.TrimSpace(opts.cloneVoiceID)
	opts.audioURL = strings.TrimSpace(opts.audioURL)
	opts.fileID = strings.TrimSpace(opts.fileID)
	opts.inputPath = strings.TrimSpace(opts.inputPath)
	opts.fileName = strings.TrimSpace(opts.fileName)
	opts.contentType = strings.TrimSpace(opts.contentType)
	opts.purpose = strings.TrimSpace(opts.purpose)

	if opts.timeout <= 0 {
		return options{}, errors.New("timeout must be greater than 0")
	}

	if opts.cloneVoiceID == "" {
		return options{}, errors.New("voice-id cannot be empty")
	}

	if opts.audioURL == "" && opts.fileID == "" && opts.inputPath == "" {
		return options{}, errors.New("clone requires one source: audio-url, file-id, or input")
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

	cloneReq := minimax.CloneVoiceRequest{
		VoiceID:  opts.cloneVoiceID,
		AudioURL: opts.audioURL,
		FileID:   opts.fileID,
	}

	var uploaded *minimax.FileUploadResponse
	if opts.inputPath != "" {
		uploadName, err := resolveUploadFileName(opts.inputPath, opts.fileName)
		if err != nil {
			return err
		}

		fileData, err := os.ReadFile(opts.inputPath)
		if err != nil {
			return fmt.Errorf("failed to read input file: %w", err)
		}

		uploaded, err = client.File.Upload(ctx, minimax.FileUploadRequest{
			Purpose:     opts.purpose,
			FileName:    uploadName,
			ContentType: opts.contentType,
			Data:        fileData,
		})
		if err != nil {
			return fmt.Errorf("File.Upload failed before cloning: %w", err)
		}

		cloneReq.FileID = strings.TrimSpace(uploaded.FileID)
		if cloneReq.FileID == "" {
			return errors.New("File.Upload succeeded but returned empty file_id")
		}
	}

	if cloneReq.AudioURL == "" && cloneReq.FileID == "" {
		return errors.New("clone request has no source after preprocessing")
	}

	response, err := client.Voice.CloneVoice(ctx, &cloneReq)
	if err != nil {
		return fmt.Errorf("Voice.CloneVoice failed: %w", err)
	}

	if opts.asJSON {
		output := struct {
			Upload *minimax.FileUploadResponse `json:"upload,omitempty"`
			Clone  *minimax.CloneVoiceResponse `json:"clone"`
		}{
			Upload: uploaded,
			Clone:  response,
		}

		payload, marshalErr := json.MarshalIndent(output, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("failed to marshal response: %w", marshalErr)
		}
		fmt.Println(string(payload))
		return nil
	}

	if uploaded != nil {
		fmt.Println("upload succeeded")
		fmt.Printf("  file_id: %s\n", uploaded.FileID)
		fmt.Printf("  file_url: %s\n", uploaded.FileURL)
	}

	fmt.Println("clone succeeded")
	fmt.Printf("  voice_id: %s\n", response.VoiceID)
	fmt.Printf("  demo_audio: %s\n", response.DemoAudio)

	return nil
}

func resolveUploadFileName(inputPath, override string) (string, error) {
	uploadName := strings.TrimSpace(override)
	if uploadName == "" {
		uploadName = filepath.Base(strings.TrimSpace(inputPath))
	}

	uploadName = strings.TrimSpace(uploadName)
	if uploadName == "" || uploadName == "." || uploadName == string(filepath.Separator) {
		return "", errors.New("resolved upload file name is empty")
	}

	return uploadName, nil
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
