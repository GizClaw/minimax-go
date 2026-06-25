package main

import (
	"context"
	"encoding/json"
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
	defaultBaseURL      = "https://api.minimax.io"
	defaultModel        = "MiniMax-Hailuo-2.3"
	defaultPrompt       = "A small robot carefully paints a glowing green circuit board on a clean desk, cinematic close-up"
	defaultDuration     = 6
	defaultResolution   = "768P"
	defaultTimeout      = 10 * time.Minute
	defaultPollInterval = 5 * time.Second
)

type options struct {
	apiKey           string
	baseURL          string
	model            string
	prompt           string
	firstFrameImage  string
	lastFrameImage   string
	subjectRefs      subjectReferenceFlags
	taskID           string
	duration         int
	resolution       string
	callbackURL      string
	promptOptimizer  bool
	fastPretreatment bool
	aigcWatermark    bool
	wait             bool
	output           string
	timeout          time.Duration
	pollInterval     time.Duration
	asJSON           bool
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
		fmt.Fprintf(os.Stderr, "video example failed: %v\n", err)
		os.Exit(1)
	}
}

func parseOptions(args []string, out io.Writer) (options, error) {
	opts := options{
		apiKey:           os.Getenv("MINIMAX_API_KEY"),
		baseURL:          envOrDefault("MINIMAX_BASE_URL", defaultBaseURL),
		model:            envOrDefault("MINIMAX_VIDEO_MODEL", defaultModel),
		prompt:           envOrDefault("MINIMAX_VIDEO_PROMPT", defaultPrompt),
		firstFrameImage:  os.Getenv("MINIMAX_VIDEO_FIRST_FRAME_IMAGE"),
		lastFrameImage:   os.Getenv("MINIMAX_VIDEO_LAST_FRAME_IMAGE"),
		taskID:           os.Getenv("MINIMAX_VIDEO_TASK_ID"),
		duration:         envIntOrDefault("MINIMAX_VIDEO_DURATION", defaultDuration),
		resolution:       envOrDefault("MINIMAX_VIDEO_RESOLUTION", defaultResolution),
		callbackURL:      os.Getenv("MINIMAX_VIDEO_CALLBACK_URL"),
		promptOptimizer:  envBoolOrDefault("MINIMAX_VIDEO_PROMPT_OPTIMIZER", true),
		fastPretreatment: envBoolOrDefault("MINIMAX_VIDEO_FAST_PRETREATMENT", false),
		aigcWatermark:    envBoolOrDefault("MINIMAX_VIDEO_AIGC_WATERMARK", false),
		wait:             envBoolOrDefault("MINIMAX_VIDEO_WAIT", false),
		output:           os.Getenv("MINIMAX_VIDEO_OUTPUT"),
		timeout:          envDurationOrDefault("MINIMAX_VIDEO_TIMEOUT", defaultTimeout),
		pollInterval:     envDurationOrDefault("MINIMAX_VIDEO_POLL_INTERVAL", defaultPollInterval),
	}

	fs := flag.NewFlagSet("video", flag.ContinueOnError)
	fs.SetOutput(out)

	if rawSubjectReference := strings.TrimSpace(os.Getenv("MINIMAX_VIDEO_SUBJECT_REFERENCE")); rawSubjectReference != "" {
		if err := opts.subjectRefs.Set(rawSubjectReference); err != nil {
			return options{}, err
		}
	}

	fs.StringVar(&opts.apiKey, "api-key", opts.apiKey, "Minimax API key (or env MINIMAX_API_KEY)")
	fs.StringVar(&opts.baseURL, "base-url", opts.baseURL, "Minimax API base URL (env: MINIMAX_BASE_URL)")
	fs.StringVar(&opts.model, "model", opts.model, "Video model for submit mode (env: MINIMAX_VIDEO_MODEL)")
	fs.StringVar(&opts.prompt, "prompt", opts.prompt, "Prompt for submit mode (env: MINIMAX_VIDEO_PROMPT)")
	fs.StringVar(&opts.firstFrameImage, "first-frame-image", opts.firstFrameImage, "First frame image URL or Data URL for image-to-video or first-last-frame submit mode")
	fs.StringVar(&opts.lastFrameImage, "last-frame-image", opts.lastFrameImage, "Last frame image URL or Data URL for first-last-frame submit mode")
	fs.Var(&opts.subjectRefs, "subject-reference", "Subject reference as type=image_url, for example character=https://example.com/person.jpg")
	fs.StringVar(&opts.taskID, "task-id", opts.taskID, "Query existing task_id instead of submitting a new task")
	fs.IntVar(&opts.duration, "duration", opts.duration, "Video duration in seconds for submit mode")
	fs.StringVar(&opts.resolution, "resolution", opts.resolution, "Video resolution for submit mode")
	fs.StringVar(&opts.callbackURL, "callback-url", opts.callbackURL, "Callback URL for submit mode")
	fs.BoolVar(&opts.promptOptimizer, "prompt-optimizer", opts.promptOptimizer, "Enable MiniMax prompt optimizer")
	fs.BoolVar(&opts.fastPretreatment, "fast-pretreatment", opts.fastPretreatment, "Enable fast pretreatment")
	fs.BoolVar(&opts.aigcWatermark, "aigc-watermark", opts.aigcWatermark, "Add AIGC watermark")
	fs.BoolVar(&opts.wait, "wait", opts.wait, "Poll until task reaches a terminal state")
	fs.StringVar(&opts.output, "output", opts.output, "Optional output path for raw video download after success")
	fs.DurationVar(&opts.timeout, "timeout", opts.timeout, "Total timeout for submit/query workflow")
	fs.DurationVar(&opts.pollInterval, "poll-interval", opts.pollInterval, "Polling interval when wait=true")
	fs.BoolVar(&opts.asJSON, "json", false, "Print final response as formatted JSON")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: go run ./examples/video [flags]\n\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nModes:\n")
		fmt.Fprintf(fs.Output(), "  - submit mode: no -task-id, creates a text-to-video task\n")
		fmt.Fprintf(fs.Output(), "  - subject-reference mode: add -subject-reference character=https://example.com/person.jpg\n")
		fmt.Fprintf(fs.Output(), "  - image-to-video mode: add -first-frame-image to submit with an initial image\n")
		fmt.Fprintf(fs.Output(), "  - first-last-frame mode: add -last-frame-image, optionally with -first-frame-image\n")
		fmt.Fprintf(fs.Output(), "  - task mode: set -task-id, queries an existing video task\n")
		fmt.Fprintf(fs.Output(), "\nNotes:\n")
		fmt.Fprintf(fs.Output(), "  - use -wait to poll until Success/Fail\n")
		fmt.Fprintf(fs.Output(), "  - use -output only when the task succeeds and returns file_id\n")
	}

	if err := fs.Parse(args); err != nil {
		return options{}, err
	}

	trimOptions(&opts)

	if opts.timeout <= 0 {
		return options{}, errors.New("timeout must be greater than 0")
	}
	if opts.pollInterval <= 0 {
		return options{}, errors.New("poll-interval must be greater than 0")
	}
	if opts.duration < 0 {
		return options{}, errors.New("duration must be non-negative")
	}
	if opts.taskID == "" {
		if opts.model == "" {
			return options{}, errors.New("submit mode requires model")
		}
		if len(opts.subjectRefs) == 0 && opts.firstFrameImage == "" && opts.lastFrameImage == "" && opts.prompt == "" {
			return options{}, errors.New("submit mode requires prompt")
		}
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

	client, err := minimax.NewClient(minimax.Config{
		BaseURL: opts.baseURL,
		APIKey:  opts.apiKey,
	})
	if err != nil {
		return fmt.Errorf("failed to create Minimax client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()

	taskID := opts.taskID
	if taskID == "" {
		submitted, submitErr := submitVideoTask(ctx, client, opts)
		if submitErr != nil {
			return submitErr
		}

		taskID = submitted.TaskID
		fmt.Fprintf(out, "submitted task_id=%s\n", taskID)
		if !opts.wait {
			return nil
		}
	}

	response, err := waitOrQuery(ctx, client, taskID, opts, out)
	if err != nil {
		return err
	}

	if opts.asJSON {
		payload, marshalErr := json.MarshalIndent(response, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("failed to marshal response: %w", marshalErr)
		}
		fmt.Fprintln(out, string(payload))
	}

	if response.FileID == "" {
		return nil
	}

	file, err := client.File.Retrieve(ctx, response.FileID)
	if err != nil {
		return fmt.Errorf("File.Retrieve failed: %w", err)
	}

	fmt.Fprintf(out, "file_id=%s\n", response.FileID)
	if file.File.DownloadURL != "" {
		fmt.Fprintf(out, "download_url=%s\n", file.File.DownloadURL)
	}

	if opts.output == "" {
		return nil
	}

	return downloadFileContent(ctx, client, response.FileID, opts.output, out)
}

func submitVideoTask(ctx context.Context, client *minimax.Client, opts options) (*minimax.VideoTaskCreateResponse, error) {
	if len(opts.subjectRefs) > 0 {
		submitted, err := client.Video.CreateSubjectReferenceVideo(ctx, minimax.VideoSubjectReferenceRequest{
			Model:             opts.model,
			SubjectReferences: opts.subjectRefs.VideoSubjectReferences(),
			Prompt:            opts.prompt,
			PromptOptimizer:   new(opts.promptOptimizer),
			CallbackURL:       opts.callbackURL,
			AIGCWatermark:     new(opts.aigcWatermark),
		})
		if err != nil {
			return nil, fmt.Errorf("Video.CreateSubjectReferenceVideo failed: %w", err)
		}
		return submitted, nil
	}

	if opts.lastFrameImage != "" {
		submitted, err := client.Video.CreateFirstLastFrameVideo(ctx, minimax.VideoFirstLastFrameRequest{
			Model:           opts.model,
			LastFrameImage:  opts.lastFrameImage,
			FirstFrameImage: opts.firstFrameImage,
			Prompt:          opts.prompt,
			PromptOptimizer: new(opts.promptOptimizer),
			Duration:        new(opts.duration),
			Resolution:      opts.resolution,
			CallbackURL:     opts.callbackURL,
			AIGCWatermark:   new(opts.aigcWatermark),
		})
		if err != nil {
			return nil, fmt.Errorf("Video.CreateFirstLastFrameVideo failed: %w", err)
		}
		return submitted, nil
	}

	if opts.firstFrameImage != "" {
		submitted, err := client.Video.CreateImageToVideo(ctx, minimax.VideoImageToVideoRequest{
			Model:            opts.model,
			FirstFrameImage:  opts.firstFrameImage,
			Prompt:           opts.prompt,
			PromptOptimizer:  new(opts.promptOptimizer),
			FastPretreatment: new(opts.fastPretreatment),
			Duration:         new(opts.duration),
			Resolution:       opts.resolution,
			CallbackURL:      opts.callbackURL,
			AIGCWatermark:    new(opts.aigcWatermark),
		})
		if err != nil {
			return nil, fmt.Errorf("Video.CreateImageToVideo failed: %w", err)
		}
		return submitted, nil
	}

	submitted, err := client.Video.CreateTextToVideo(ctx, minimax.VideoTextToVideoRequest{
		Model:            opts.model,
		Prompt:           opts.prompt,
		PromptOptimizer:  new(opts.promptOptimizer),
		FastPretreatment: new(opts.fastPretreatment),
		Duration:         new(opts.duration),
		Resolution:       opts.resolution,
		CallbackURL:      opts.callbackURL,
		AIGCWatermark:    new(opts.aigcWatermark),
	})
	if err != nil {
		return nil, fmt.Errorf("Video.CreateTextToVideo failed: %w", err)
	}
	return submitted, nil
}

func waitOrQuery(ctx context.Context, client *minimax.Client, taskID string, opts options, out io.Writer) (*minimax.VideoTaskStatusResponse, error) {
	for {
		response, err := client.Video.GetTask(ctx, taskID)
		if err != nil {
			return nil, fmt.Errorf("Video.GetTask failed: %w", err)
		}

		fmt.Fprintf(out, "task_id=%s status=%s raw_status=%s file_id=%s\n", response.TaskID, response.Status, response.RawStatus, response.FileID)
		if !opts.wait || response.Status.IsTerminal() {
			return response, nil
		}

		timer := time.NewTimer(opts.pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func trimOptions(opts *options) {
	opts.apiKey = strings.TrimSpace(opts.apiKey)
	opts.baseURL = strings.TrimSpace(opts.baseURL)
	opts.model = strings.TrimSpace(opts.model)
	opts.prompt = strings.TrimSpace(opts.prompt)
	opts.firstFrameImage = strings.TrimSpace(opts.firstFrameImage)
	opts.lastFrameImage = strings.TrimSpace(opts.lastFrameImage)
	opts.subjectRefs.Trim()
	opts.taskID = strings.TrimSpace(opts.taskID)
	opts.resolution = strings.TrimSpace(opts.resolution)
	opts.callbackURL = strings.TrimSpace(opts.callbackURL)
	opts.output = strings.TrimSpace(opts.output)
}

func downloadFileContent(ctx context.Context, client *minimax.Client, fileID string, output string, out io.Writer) error {
	downloaded, err := client.File.Download(ctx, fileID)
	if err != nil {
		return fmt.Errorf("File.Download failed: %w", err)
	}
	defer downloaded.Body.Close()

	if err := writeBodyToFile(downloaded.Body, output); err != nil {
		return err
	}

	fmt.Fprintf(out, "saved=%s\n", output)
	return nil
}

func writeBodyToFile(body io.Reader, output string) error {
	outputFile, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	if _, err := io.Copy(outputFile, body); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

type subjectReferenceFlags []subjectReferenceFlag

type subjectReferenceFlag struct {
	referenceType string
	imageURL      string
}

func (f *subjectReferenceFlags) String() string {
	if f == nil || len(*f) == 0 {
		return ""
	}

	values := make([]string, 0, len(*f))
	for _, reference := range *f {
		values = append(values, reference.referenceType+"="+reference.imageURL)
	}
	return strings.Join(values, ",")
}

func (f *subjectReferenceFlags) Set(value string) error {
	referenceType, imageURL, ok := strings.Cut(value, "=")
	referenceType = strings.TrimSpace(referenceType)
	imageURL = strings.TrimSpace(imageURL)
	if !ok || referenceType == "" || imageURL == "" {
		return errors.New("subject-reference must be formatted as type=image_url")
	}

	*f = append(*f, subjectReferenceFlag{
		referenceType: referenceType,
		imageURL:      imageURL,
	})
	return nil
}

func (f *subjectReferenceFlags) Trim() {
	for index := range *f {
		(*f)[index].referenceType = strings.TrimSpace((*f)[index].referenceType)
		(*f)[index].imageURL = strings.TrimSpace((*f)[index].imageURL)
	}
}

func (f subjectReferenceFlags) VideoSubjectReferences() []minimax.VideoSubjectReference {
	references := make([]minimax.VideoSubjectReference, 0, len(f))
	for _, reference := range f {
		references = append(references, minimax.VideoSubjectReference{
			Type:  reference.referenceType,
			Image: []string{reference.imageURL},
		})
	}
	return references
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

func envIntOrDefault(key string, defaultValue int) int {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return defaultValue
	}

	var parsed int
	if _, err := fmt.Sscanf(strings.TrimSpace(raw), "%d", &parsed); err != nil {
		return defaultValue
	}

	return parsed
}

func envBoolOrDefault(key string, defaultValue bool) bool {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return defaultValue
	}

	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	case "0", "f", "false", "n", "no", "off":
		return false
	default:
		return defaultValue
	}
}
