package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	minimax "github.com/GizClaw/minimax-go"
)

type options struct {
	file, model, format, language, baseURL, output string
	stream                                         bool
	timeout                                        time.Duration
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	var opts options
	flags := flag.NewFlagSet("transcription", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&opts.file, "file", "", "Audio file (required; max 50 MiB and 500 seconds)")
	flags.StringVar(&opts.model, "model", "asr-1.0", "Transcription model")
	flags.StringVar(&opts.format, "format", "json", "json, verbose_json, srt or vtt")
	flags.StringVar(&opts.language, "language", "", "Optional BCP-47 language hint; empty enables mixed languages")
	flags.StringVar(&opts.baseURL, "base-url", "https://api.minimaxi.com", "MiniMax API base URL")
	flags.StringVar(&opts.output, "output", "", "Output file; default stdout")
	flags.BoolVar(&opts.stream, "stream", false, "Stream SSE events as JSON lines (requires json format)")
	flags.DurationVar(&opts.timeout, "timeout", 10*time.Minute, "Timeout for upload and complete transcription")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if opts.file == "" {
		return errors.New("-file is required")
	}
	if opts.timeout <= 0 {
		return errors.New("-timeout must be positive")
	}
	key := os.Getenv("MINIMAX_API_KEY")
	if key == "" {
		return errors.New("MINIMAX_API_KEY is required")
	}
	file, err := os.Open(opts.file)
	if err != nil {
		return fmt.Errorf("open audio: %w", err)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, minimax.SpeechToTextMaxFileBytes+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return fmt.Errorf("read audio: %w", err)
	}
	client, err := minimax.NewClient(minimax.Config{APIKey: key, BaseURL: opts.baseURL, HTTPClient: &http.Client{Timeout: opts.timeout}})
	if err != nil {
		return err
	}
	request := minimax.SpeechToTextRequest{Model: minimax.SpeechToTextModel(opts.model), File: minimax.SpeechToTextFile{Name: opts.file, Data: data}, ResponseFormat: minimax.SpeechToTextFormat(opts.format), Language: opts.language}
	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()
	// Finish validation and open the response before creating an output file.
	if opts.stream {
		stream, err := client.SpeechToText.OpenStream(ctx, request)
		if err != nil {
			return err
		}
		return errors.Join(writeOutput(opts.output, stdout, func(w io.Writer) error {
			encoder := json.NewEncoder(w)
			for {
				chunk, err := stream.Next()
				if errors.Is(err, io.EOF) {
					return nil
				}
				if err != nil {
					return err
				}
				if err := encoder.Encode(chunk); err != nil {
					return err
				}
			}
		}), stream.Close())
	}
	result, err := client.SpeechToText.Transcribe(ctx, request)
	if err != nil {
		return err
	}
	return writeOutput(opts.output, stdout, func(w io.Writer) error {
		if opts.format == "srt" || opts.format == "vtt" {
			_, err := io.WriteString(w, result.Subtitles)
			return err
		}
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	})
}

func writeOutput(path string, stdout io.Writer, write func(io.Writer) error) error {
	if path == "" {
		return write(stdout)
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	return errors.Join(write(file), file.Close())
}
