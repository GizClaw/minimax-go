package main

import (
	"context"
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
	defaultPurpose = "voice_clone"
	defaultTimeout = 30 * time.Second
)

type options struct {
	apiKey      string
	baseURL     string
	inputPath   string
	fileName    string
	contentType string
	purpose     string
	timeout     time.Duration
}

func main() {
	opts, err := parseOptions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse flags: %v\n", err)
		os.Exit(2)
	}

	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "file upload example failed: %v\n", err)
		os.Exit(1)
	}
}

func parseOptions() (options, error) {
	var opts options

	apiKeyDefault := os.Getenv("MINIMAX_API_KEY")
	baseURLDefault := envOrDefault("MINIMAX_BASE_URL", defaultBaseURL)
	inputPathDefault := os.Getenv("MINIMAX_FILE_INPUT")
	fileNameDefault := os.Getenv("MINIMAX_FILE_NAME")
	contentTypeDefault := os.Getenv("MINIMAX_FILE_CONTENT_TYPE")
	purposeDefault := envOrDefault("MINIMAX_FILE_PURPOSE", defaultPurpose)
	timeoutDefault := envDurationOrDefault("MINIMAX_FILE_TIMEOUT", defaultTimeout)

	flag.StringVar(&opts.apiKey, "api-key", apiKeyDefault, "Minimax API key (or env MINIMAX_API_KEY)")
	flag.StringVar(&opts.baseURL, "base-url", baseURLDefault, "Minimax API base URL (env: MINIMAX_BASE_URL)")
	flag.StringVar(&opts.inputPath, "input", inputPathDefault, "Local file path to upload (env: MINIMAX_FILE_INPUT)")
	flag.StringVar(&opts.fileName, "file-name", fileNameDefault, "Uploaded file name override (optional, env: MINIMAX_FILE_NAME)")
	flag.StringVar(&opts.contentType, "content-type", contentTypeDefault, "MIME type override (optional, env: MINIMAX_FILE_CONTENT_TYPE)")
	flag.StringVar(&opts.purpose, "purpose", purposeDefault, "File purpose field (optional, env: MINIMAX_FILE_PURPOSE)")
	flag.DurationVar(&opts.timeout, "timeout", timeoutDefault, "Request timeout (env: MINIMAX_FILE_TIMEOUT, e.g. 30s)")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: go run ./examples/file [flags]\n\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nNotes:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  - API key precedence: -api-key > MINIMAX_API_KEY\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  - If -file-name is empty, base name of -input is used\n")
	}

	flag.Parse()

	opts.apiKey = strings.TrimSpace(opts.apiKey)
	opts.baseURL = strings.TrimSpace(opts.baseURL)
	opts.inputPath = strings.TrimSpace(opts.inputPath)
	opts.fileName = strings.TrimSpace(opts.fileName)
	opts.contentType = strings.TrimSpace(opts.contentType)
	opts.purpose = strings.TrimSpace(opts.purpose)

	if opts.timeout <= 0 {
		return options{}, errors.New("timeout must be greater than 0")
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

	if opts.inputPath == "" {
		return errors.New("input cannot be empty")
	}

	fileData, err := os.ReadFile(opts.inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	uploadedName := opts.fileName
	if uploadedName == "" {
		uploadedName = filepath.Base(opts.inputPath)
	}

	uploadedName = strings.TrimSpace(uploadedName)
	if uploadedName == "" || uploadedName == "." || uploadedName == string(filepath.Separator) {
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

	response, err := client.File.Upload(ctx, minimax.FileUploadRequest{
		Purpose:     opts.purpose,
		FileName:    uploadedName,
		ContentType: opts.contentType,
		Data:        fileData,
	})
	if err != nil {
		return fmt.Errorf("File.Upload failed: %w", err)
	}

	fmt.Printf("upload succeeded\n")
	fmt.Printf("  uploaded: %t\n", response.Uploaded)
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
	if err != nil {
		return defaultValue
	}

	if parsed <= 0 {
		return defaultValue
	}

	return parsed
}
