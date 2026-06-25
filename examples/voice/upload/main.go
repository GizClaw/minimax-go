package main

import (
	"bytes"
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
	defaultBaseURL = "https://api.minimax.io"
	defaultTimeout = 30 * time.Second
)

type options struct {
	apiKey      string
	baseURL     string
	kind        string
	inputPath   string
	fileName    string
	contentType string
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
		fmt.Fprintf(os.Stderr, "voice upload example failed: %v\n", err)
		os.Exit(1)
	}
}

func parseOptions() (options, error) {
	var opts options

	flag.StringVar(&opts.apiKey, "api-key", os.Getenv("MINIMAX_API_KEY"), "Minimax API key (or env MINIMAX_API_KEY)")
	flag.StringVar(&opts.baseURL, "base-url", envOrDefault("MINIMAX_BASE_URL", defaultBaseURL), "Minimax API base URL (env: MINIMAX_BASE_URL)")
	flag.StringVar(&opts.kind, "kind", envOrDefault("MINIMAX_VOICE_UPLOAD_KIND", "clone"), "Upload helper kind: clone or prompt (env: MINIMAX_VOICE_UPLOAD_KIND)")
	flag.StringVar(&opts.inputPath, "input", os.Getenv("MINIMAX_VOICE_UPLOAD_INPUT"), "Local audio path to upload (env: MINIMAX_VOICE_UPLOAD_INPUT)")
	flag.StringVar(&opts.fileName, "file-name", os.Getenv("MINIMAX_VOICE_UPLOAD_FILE_NAME"), "Uploaded file name override (env: MINIMAX_VOICE_UPLOAD_FILE_NAME)")
	flag.StringVar(&opts.contentType, "content-type", os.Getenv("MINIMAX_VOICE_UPLOAD_CONTENT_TYPE"), "MIME type override (env: MINIMAX_VOICE_UPLOAD_CONTENT_TYPE)")
	flag.DurationVar(&opts.timeout, "timeout", envDurationOrDefault("MINIMAX_VOICE_UPLOAD_TIMEOUT", defaultTimeout), "Request timeout (env: MINIMAX_VOICE_UPLOAD_TIMEOUT, e.g. 30s)")
	flag.BoolVar(&opts.asJSON, "json", false, "Print response as formatted JSON")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: go run ./examples/voice/upload -kind clone|prompt -input <audio> [flags]\n\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	opts.apiKey = strings.TrimSpace(opts.apiKey)
	opts.baseURL = strings.TrimSpace(opts.baseURL)
	opts.kind = strings.TrimSpace(opts.kind)
	opts.inputPath = strings.TrimSpace(opts.inputPath)
	opts.fileName = strings.TrimSpace(opts.fileName)
	opts.contentType = strings.TrimSpace(opts.contentType)

	if opts.timeout <= 0 {
		return options{}, errors.New("timeout must be greater than 0")
	}
	if opts.kind != "clone" && opts.kind != "prompt" {
		return options{}, errors.New("kind must be clone or prompt")
	}
	if opts.inputPath == "" {
		return options{}, errors.New("input cannot be empty")
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

	fileData, err := os.ReadFile(opts.inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	uploadName := strings.TrimSpace(opts.fileName)
	if uploadName == "" {
		uploadName = filepath.Base(opts.inputPath)
	}
	if uploadName == "" || uploadName == "." || uploadName == string(filepath.Separator) {
		return errors.New("resolved uploaded file name is empty")
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

	var response *minimax.FileUploadResponse
	switch opts.kind {
	case "clone":
		response, err = client.Voice.UploadCloneAudio(ctx, minimax.UploadCloneAudioRequest{
			Filename:    uploadName,
			Content:     bytes.NewReader(fileData),
			ContentType: opts.contentType,
		})
	case "prompt":
		response, err = client.Voice.UploadPromptAudio(ctx, minimax.UploadPromptAudioRequest{
			Filename:    uploadName,
			Content:     bytes.NewReader(fileData),
			ContentType: opts.contentType,
		})
	}
	if err != nil {
		return fmt.Errorf("Voice upload helper failed: %w", err)
	}

	if opts.asJSON {
		payload, marshalErr := json.MarshalIndent(response, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("failed to marshal response: %w", marshalErr)
		}
		fmt.Println(string(payload))
		return nil
	}

	fmt.Println("upload succeeded")
	fmt.Printf("  kind: %s\n", opts.kind)
	fmt.Printf("  file_id: %s\n", response.FileID)
	fmt.Printf("  file_url: %s\n", response.FileURL)
	fmt.Printf("  meta.file_name: %s\n", response.Meta.FileName)
	fmt.Printf("  meta.content_type: %s\n", response.Meta.ContentType)
	fmt.Printf("  meta.size: %d\n", response.Meta.Size)
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
