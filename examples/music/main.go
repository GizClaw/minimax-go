package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	minimax "github.com/GizClaw/minimax-go"
)

const (
	defaultBaseURL      = "https://api.minimaxi.com"
	defaultMode         = "generate"
	defaultModel        = "music-2.6-free"
	defaultCoverModel   = "music-cover-free"
	defaultPrompt       = "bright chiptune, playful, short game theme"
	defaultLyrics       = "[Verse]\nTiny lights wake up the room\nLittle circuits hum a tune\n[Chorus]\nBuild it bright and let it play\nGreen sparks dancing through the day"
	defaultOutputFormat = "url"
	defaultSampleRate   = 44100
	defaultBitrate      = 256000
	defaultAudioFormat  = "mp3"
	defaultTimeout      = 5 * time.Minute
)

type options struct {
	mode            string
	apiKey          string
	baseURL         string
	model           string
	lyricsMode      string
	prompt          string
	lyrics          string
	lyricsFile      string
	title           string
	outputFormat    string
	sampleRate      int
	bitrate         int
	audioFormat     string
	aigcWatermark   bool
	lyricsOptimizer bool
	instrumental    bool
	audioURL        string
	audioBase64     string
	coverFeatureID  string
	output          string
	timeout         time.Duration
	asJSON          bool
}

func main() {
	opts, err := parseOptions(os.Args[1:], os.Stderr)
	if err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintf(os.Stderr, "failed to parse flags: %v\n", err)
			os.Exit(2)
		}
		return
	}

	if err := run(opts, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "music example failed: %v\n", err)
		os.Exit(1)
	}
}

func parseOptions(args []string, out io.Writer) (options, error) {
	if len(args) == 0 {
		args = []string{defaultMode}
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		printTopLevelUsage(out)
		return options{}, flag.ErrHelp
	}

	mode := strings.TrimSpace(args[0])
	switch mode {
	case "lyrics", "generate", "preprocess", "cover":
	default:
		return options{}, fmt.Errorf("unknown mode %q", mode)
	}

	opts := options{
		mode:            mode,
		apiKey:          os.Getenv("MINIMAX_API_KEY"),
		baseURL:         envOrDefault("MINIMAX_BASE_URL", defaultBaseURL),
		model:           envOrDefault("MINIMAX_MUSIC_MODEL", defaultModel),
		lyricsMode:      envOrDefault("MINIMAX_MUSIC_LYRICS_MODE", string(minimax.LyricsModeWriteFullSong)),
		prompt:          envOrDefault("MINIMAX_MUSIC_PROMPT", defaultPrompt),
		lyrics:          envOrDefault("MINIMAX_MUSIC_LYRICS", defaultLyrics),
		lyricsFile:      os.Getenv("MINIMAX_MUSIC_LYRICS_FILE"),
		title:           os.Getenv("MINIMAX_MUSIC_TITLE"),
		outputFormat:    envOrDefault("MINIMAX_MUSIC_OUTPUT_FORMAT", defaultOutputFormat),
		sampleRate:      envIntOrDefault("MINIMAX_MUSIC_SAMPLE_RATE", defaultSampleRate),
		bitrate:         envIntOrDefault("MINIMAX_MUSIC_BITRATE", defaultBitrate),
		audioFormat:     envOrDefault("MINIMAX_MUSIC_AUDIO_FORMAT", defaultAudioFormat),
		aigcWatermark:   envBoolOrDefault("MINIMAX_MUSIC_AIGC_WATERMARK", false),
		lyricsOptimizer: envBoolOrDefault("MINIMAX_MUSIC_LYRICS_OPTIMIZER", false),
		instrumental:    envBoolOrDefault("MINIMAX_MUSIC_INSTRUMENTAL", false),
		audioURL:        os.Getenv("MINIMAX_MUSIC_AUDIO_URL"),
		audioBase64:     os.Getenv("MINIMAX_MUSIC_AUDIO_BASE64"),
		coverFeatureID:  os.Getenv("MINIMAX_MUSIC_COVER_FEATURE_ID"),
		output:          os.Getenv("MINIMAX_MUSIC_OUTPUT"),
		timeout:         envDurationOrDefault("MINIMAX_MUSIC_TIMEOUT", defaultTimeout),
	}
	if mode == "cover" && opts.model == defaultModel {
		opts.model = defaultCoverModel
	}
	if mode == "preprocess" {
		opts.model = string(minimax.MusicModelCover)
	}

	fs := flag.NewFlagSet("music "+mode, flag.ContinueOnError)
	fs.SetOutput(out)
	fs.StringVar(&opts.apiKey, "api-key", opts.apiKey, "MiniMax API key (or env MINIMAX_API_KEY)")
	fs.StringVar(&opts.baseURL, "base-url", opts.baseURL, "MiniMax API base URL (env: MINIMAX_BASE_URL)")
	fs.StringVar(&opts.model, "model", opts.model, "Music model for generate/cover/preprocess")
	fs.StringVar(&opts.lyricsMode, "lyrics-mode", opts.lyricsMode, "Lyrics generation mode: write_full_song or edit")
	fs.StringVar(&opts.prompt, "prompt", opts.prompt, "Music prompt or lyrics instruction")
	fs.StringVar(&opts.lyrics, "lyrics", opts.lyrics, "Lyrics text for generate/cover/edit")
	fs.StringVar(&opts.lyricsFile, "lyrics-file", opts.lyricsFile, "Read lyrics from file")
	fs.StringVar(&opts.title, "title", opts.title, "Optional song title for lyrics generation")
	fs.StringVar(&opts.outputFormat, "output-format", opts.outputFormat, "Music output format: url or hex")
	fs.IntVar(&opts.sampleRate, "sample-rate", opts.sampleRate, "Audio sample rate")
	fs.IntVar(&opts.bitrate, "bitrate", opts.bitrate, "Audio bitrate")
	fs.StringVar(&opts.audioFormat, "audio-format", opts.audioFormat, "Audio format, for example mp3 or wav")
	fs.BoolVar(&opts.aigcWatermark, "aigc-watermark", opts.aigcWatermark, "Add AIGC watermark")
	fs.BoolVar(&opts.lyricsOptimizer, "lyrics-optimizer", opts.lyricsOptimizer, "Let MiniMax generate lyrics from prompt")
	fs.BoolVar(&opts.instrumental, "instrumental", opts.instrumental, "Generate instrumental music without lyrics")
	fs.StringVar(&opts.audioURL, "audio-url", opts.audioURL, "Reference audio URL for cover/preprocess")
	fs.StringVar(&opts.audioBase64, "audio-base64", opts.audioBase64, "Reference audio base64 for cover/preprocess")
	fs.StringVar(&opts.coverFeatureID, "cover-feature-id", opts.coverFeatureID, "Feature ID returned by preprocess mode")
	fs.StringVar(&opts.output, "output", opts.output, "Optional local output file for returned URL or hex audio")
	fs.DurationVar(&opts.timeout, "timeout", opts.timeout, "Request timeout")
	fs.BoolVar(&opts.asJSON, "json", false, "Print full response as formatted JSON")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: go run ./examples/music %s [flags]\n\n", mode)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args[1:]); err != nil {
		return options{}, err
	}

	trimOptions(&opts)
	if opts.timeout <= 0 {
		return options{}, errors.New("timeout must be greater than 0")
	}
	if opts.mode != "lyrics" && opts.model == "" {
		return options{}, errors.New("model is required")
	}
	if opts.mode == "lyrics" && opts.lyricsMode == "" {
		return options{}, errors.New("lyrics-mode is required")
	}
	if opts.sampleRate < 0 {
		return options{}, errors.New("sample-rate must be non-negative")
	}
	if opts.bitrate < 0 {
		return options{}, errors.New("bitrate must be non-negative")
	}

	return opts, nil
}

func run(opts options, out io.Writer) error {
	if opts.apiKey == "" {
		return errors.New("missing API key: use -api-key or set MINIMAX_API_KEY")
	}
	if opts.baseURL == "" {
		return errors.New("base-url cannot be empty")
	}
	if opts.lyricsFile != "" {
		lyrics, err := os.ReadFile(opts.lyricsFile)
		if err != nil {
			return fmt.Errorf("read lyrics file: %w", err)
		}
		opts.lyrics = strings.TrimSpace(string(lyrics))
	}

	client, err := minimax.NewClient(minimax.Config{
		BaseURL:    opts.baseURL,
		APIKey:     opts.apiKey,
		HTTPClient: &http.Client{Timeout: opts.timeout},
	})
	if err != nil {
		return fmt.Errorf("failed to create MiniMax client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()

	switch opts.mode {
	case "lyrics":
		return runLyrics(ctx, client, opts, out)
	case "generate":
		return runGenerate(ctx, client, opts, out, false)
	case "preprocess":
		return runPreprocess(ctx, client, opts, out)
	case "cover":
		return runGenerate(ctx, client, opts, out, true)
	default:
		return fmt.Errorf("unknown mode %q", opts.mode)
	}
}

func runLyrics(ctx context.Context, client *minimax.Client, opts options, out io.Writer) error {
	response, err := client.Music.GenerateLyrics(ctx, minimax.LyricsGenerateRequest{
		Mode:   opts.lyricsMode,
		Prompt: opts.prompt,
		Lyrics: opts.lyrics,
		Title:  opts.title,
	})
	if err != nil {
		return fmt.Errorf("Music.GenerateLyrics failed: %w", err)
	}
	if opts.asJSON {
		if err := printJSON(out, response); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "song_title=%s\n", response.SongTitle)
	fmt.Fprintf(out, "style_tags=%s\n", response.StyleTags)
	fmt.Fprintf(out, "lyrics=%s\n", response.Lyrics)
	return nil
}

func runGenerate(ctx context.Context, client *minimax.Client, opts options, out io.Writer, cover bool) error {
	request := minimax.MusicGenerateRequest{
		Model:         opts.model,
		Prompt:        opts.prompt,
		Lyrics:        opts.lyrics,
		OutputFormat:  opts.outputFormat,
		AudioSetting:  buildAudioSetting(opts),
		AIGCWatermark: new(opts.aigcWatermark),
	}
	if cover {
		request.AudioURL = opts.audioURL
		request.AudioBase64 = opts.audioBase64
		request.CoverFeatureID = opts.coverFeatureID
	} else {
		if opts.lyricsOptimizer {
			request.LyricsOptimizer = new(opts.lyricsOptimizer)
		}
		if opts.instrumental {
			request.IsInstrumental = new(opts.instrumental)
		}
	}

	response, err := client.Music.Generate(ctx, request)
	if err != nil {
		return fmt.Errorf("Music.Generate failed: %w", err)
	}
	if opts.asJSON {
		if err := printJSON(out, response); err != nil {
			return err
		}
	}
	printGenerateSummary(out, response)
	if opts.output != "" {
		if err := saveAudio(ctx, response.Audio, opts.output); err != nil {
			return err
		}
		fmt.Fprintf(out, "saved=%s\n", opts.output)
	}
	return nil
}

func runPreprocess(ctx context.Context, client *minimax.Client, opts options, out io.Writer) error {
	response, err := client.Music.PreprocessCover(ctx, minimax.MusicCoverPreprocessRequest{
		Model:       opts.model,
		AudioURL:    opts.audioURL,
		AudioBase64: opts.audioBase64,
	})
	if err != nil {
		return fmt.Errorf("Music.PreprocessCover failed: %w", err)
	}
	if opts.asJSON {
		if err := printJSON(out, response); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "cover_feature_id=%s\n", response.CoverFeatureID)
	fmt.Fprintf(out, "audio_duration=%s\n", formatOptionalFloat(response.AudioDuration))
	if response.FormattedLyrics != "" {
		fmt.Fprintf(out, "formatted_lyrics=%s\n", response.FormattedLyrics)
	}
	return nil
}

func buildAudioSetting(opts options) *minimax.MusicAudioSetting {
	if opts.sampleRate == 0 && opts.bitrate == 0 && opts.audioFormat == "" {
		return nil
	}
	setting := &minimax.MusicAudioSetting{Format: opts.audioFormat}
	if opts.sampleRate > 0 {
		setting.SampleRate = new(opts.sampleRate)
	}
	if opts.bitrate > 0 {
		setting.Bitrate = new(opts.bitrate)
	}
	return setting
}

func printGenerateSummary(out io.Writer, response *minimax.MusicGenerateResponse) {
	fmt.Fprintf(out, "audio=%s\n", response.Audio)
	if response.Status != nil {
		fmt.Fprintf(out, "status=%d\n", *response.Status)
	}
	if response.ExtraInfo.MusicDuration != nil || response.ExtraInfo.MusicSize != nil {
		fmt.Fprintf(out, "duration_ms=%s size_bytes=%s\n", formatOptionalInt(response.ExtraInfo.MusicDuration), formatOptionalInt(response.ExtraInfo.MusicSize))
	}
	if response.TraceID != "" {
		fmt.Fprintf(out, "trace_id=%s\n", response.TraceID)
	}
}

func saveAudio(ctx context.Context, audio string, path string) error {
	audio = strings.TrimSpace(audio)
	if audio == "" {
		return errors.New("response audio is empty")
	}
	if isHTTPURL(audio) {
		return downloadURL(ctx, audio, path)
	}
	data, err := hex.DecodeString(audio)
	if err != nil {
		return fmt.Errorf("decode hex audio: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func downloadURL(ctx context.Context, rawURL string, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download audio: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download audio: status %d", resp.StatusCode)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}
	return nil
}

func isHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func printJSON(out io.Writer, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	fmt.Fprintln(out, string(payload))
	return nil
}

func printTopLevelUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage: go run ./examples/music <mode> [flags]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Modes:")
	fmt.Fprintln(out, "  lyrics      generate or edit lyrics")
	fmt.Fprintln(out, "  generate    generate music")
	fmt.Fprintln(out, "  preprocess  preprocess cover reference audio")
	fmt.Fprintln(out, "  cover       generate one-step or two-step cover music")
}

func trimOptions(opts *options) {
	opts.apiKey = strings.TrimSpace(opts.apiKey)
	opts.baseURL = strings.TrimSpace(opts.baseURL)
	opts.mode = strings.TrimSpace(opts.mode)
	opts.model = strings.TrimSpace(opts.model)
	opts.lyricsMode = strings.TrimSpace(opts.lyricsMode)
	opts.prompt = strings.TrimSpace(opts.prompt)
	opts.lyrics = strings.TrimSpace(opts.lyrics)
	opts.lyricsFile = strings.TrimSpace(opts.lyricsFile)
	opts.title = strings.TrimSpace(opts.title)
	opts.outputFormat = strings.TrimSpace(opts.outputFormat)
	opts.audioFormat = strings.TrimSpace(opts.audioFormat)
	opts.audioURL = strings.TrimSpace(opts.audioURL)
	opts.audioBase64 = strings.TrimSpace(opts.audioBase64)
	opts.coverFeatureID = strings.TrimSpace(opts.coverFeatureID)
	opts.output = strings.TrimSpace(opts.output)
}

func formatOptionalInt(value *int) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *value)
}

func formatOptionalFloat(value *float64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%.3f", *value)
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBoolOrDefault(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDurationOrDefault(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envIntOrDefault(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
