package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	minimax "github.com/GizClaw/minimax-go"
)

const (
	defaultBaseURL = "https://api.minimax.io"
	defaultModel   = "image-01"
	defaultPrompt  = "A tiny desktop robot drawing a green circuit board, clean product photography"
	defaultTimeout = 2 * time.Minute
)

type options struct {
	apiKey          string
	baseURL         string
	model           string
	prompt          string
	styleType       string
	styleWeight     float64
	aspectRatio     string
	width           int
	height          int
	responseFormat  string
	seed            int64
	n               int
	subjectRefs     subjectReferenceFlags
	promptOptimizer bool
	aigcWatermark   bool
	outputDir       string
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
		fmt.Fprintf(os.Stderr, "image example failed: %v\n", err)
		os.Exit(1)
	}
}

func parseOptions(args []string, out io.Writer) (options, error) {
	opts := options{
		apiKey:          os.Getenv("MINIMAX_API_KEY"),
		baseURL:         envOrDefault("MINIMAX_BASE_URL", defaultBaseURL),
		model:           envOrDefault("MINIMAX_IMAGE_MODEL", defaultModel),
		prompt:          envOrDefault("MINIMAX_IMAGE_PROMPT", defaultPrompt),
		styleType:       os.Getenv("MINIMAX_IMAGE_STYLE_TYPE"),
		aspectRatio:     envOrDefault("MINIMAX_IMAGE_ASPECT_RATIO", "1:1"),
		width:           envIntOrDefault("MINIMAX_IMAGE_WIDTH", 0),
		height:          envIntOrDefault("MINIMAX_IMAGE_HEIGHT", 0),
		responseFormat:  envOrDefault("MINIMAX_IMAGE_RESPONSE_FORMAT", "url"),
		seed:            envInt64OrDefault("MINIMAX_IMAGE_SEED", 0),
		n:               envIntOrDefault("MINIMAX_IMAGE_N", 1),
		promptOptimizer: envBoolOrDefault("MINIMAX_IMAGE_PROMPT_OPTIMIZER", false),
		aigcWatermark:   envBoolOrDefault("MINIMAX_IMAGE_AIGC_WATERMARK", false),
		outputDir:       os.Getenv("MINIMAX_IMAGE_OUTPUT_DIR"),
		timeout:         envDurationOrDefault("MINIMAX_IMAGE_TIMEOUT", defaultTimeout),
	}
	opts.styleWeight = envFloatOrDefault("MINIMAX_IMAGE_STYLE_WEIGHT", 0)

	fs := flag.NewFlagSet("image", flag.ContinueOnError)
	fs.SetOutput(out)

	fs.StringVar(&opts.apiKey, "api-key", opts.apiKey, "MiniMax API key (or env MINIMAX_API_KEY)")
	fs.StringVar(&opts.baseURL, "base-url", opts.baseURL, "MiniMax API base URL (env: MINIMAX_BASE_URL)")
	fs.StringVar(&opts.model, "model", opts.model, "Image model, for example image-01 or image-01-live")
	fs.StringVar(&opts.prompt, "prompt", opts.prompt, "Text prompt")
	fs.StringVar(&opts.styleType, "style-type", opts.styleType, "Optional style type for image-01-live")
	fs.Float64Var(&opts.styleWeight, "style-weight", opts.styleWeight, "Optional style weight for image-01-live")
	fs.StringVar(&opts.aspectRatio, "aspect-ratio", opts.aspectRatio, "Aspect ratio such as 1:1, 16:9, or 9:16")
	fs.IntVar(&opts.width, "width", opts.width, "Optional width; must be used with -height")
	fs.IntVar(&opts.height, "height", opts.height, "Optional height; must be used with -width")
	fs.StringVar(&opts.responseFormat, "response-format", opts.responseFormat, "Response format: url or base64")
	fs.Int64Var(&opts.seed, "seed", opts.seed, "Optional deterministic seed")
	fs.IntVar(&opts.n, "n", opts.n, "Number of images, 1..9")
	fs.Var(&opts.subjectRefs, "subject-reference", "Image-to-image subject reference as type=image_file; repeat for multiple references")
	fs.BoolVar(&opts.promptOptimizer, "prompt-optimizer", opts.promptOptimizer, "Enable MiniMax prompt optimizer")
	fs.BoolVar(&opts.aigcWatermark, "aigc-watermark", opts.aigcWatermark, "Add AIGC watermark")
	fs.StringVar(&opts.outputDir, "output-dir", opts.outputDir, "Optional directory for base64 image outputs")
	fs.DurationVar(&opts.timeout, "timeout", opts.timeout, "Request timeout")
	fs.BoolVar(&opts.asJSON, "json", false, "Print full response as formatted JSON")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: go run ./examples/image [flags]\n\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nNotes:\n")
		fmt.Fprintf(fs.Output(), "  - default response format is url; signed URLs are temporary\n")
		fmt.Fprintf(fs.Output(), "  - use -response-format base64 with -output-dir to save returned images\n")
		fmt.Fprintf(fs.Output(), "  - use -subject-reference character=https://example.com/ref.png to run image-to-image\n")
	}

	if err := fs.Parse(args); err != nil {
		return options{}, err
	}

	trimOptions(&opts)
	if opts.timeout <= 0 {
		return options{}, errors.New("timeout must be greater than 0")
	}
	if opts.model == "" {
		return options{}, errors.New("model is required")
	}
	if opts.prompt == "" {
		return options{}, errors.New("prompt is required")
	}
	if (opts.width == 0) != (opts.height == 0) {
		return options{}, errors.New("width and height must be provided together")
	}
	if opts.n < 1 || opts.n > 9 {
		return options{}, errors.New("n must be between 1 and 9")
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
		return fmt.Errorf("failed to create MiniMax client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()

	response, err := generateImage(ctx, client, opts)
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

	printResponseSummary(out, response)
	if opts.outputDir != "" && len(response.ImageBase64) > 0 {
		if err := saveBase64Images(response.ImageBase64, opts.outputDir, out); err != nil {
			return err
		}
	}

	return nil
}

func generateImage(ctx context.Context, client *minimax.Client, opts options) (*minimax.ImageGenerationResponse, error) {
	if len(opts.subjectRefs) > 0 {
		request := minimax.ImageImageToImageRequest{
			Model:             opts.model,
			Prompt:            opts.prompt,
			SubjectReferences: opts.subjectRefs.ImageSubjectReferences(),
			AspectRatio:       opts.aspectRatio,
			ResponseFormat:    opts.responseFormat,
			N:                 intPtr(opts.n),
			PromptOptimizer:   boolPtr(opts.promptOptimizer),
			AIGCWatermark:     boolPtr(opts.aigcWatermark),
		}
		if opts.styleType != "" {
			request.Style = &minimax.ImageStyle{StyleType: opts.styleType}
			if opts.styleWeight > 0 {
				request.Style.StyleWeight = floatPtr(opts.styleWeight)
			}
		}
		if opts.width != 0 {
			request.Width = intPtr(opts.width)
			request.Height = intPtr(opts.height)
		}
		if opts.seed != 0 {
			request.Seed = int64Ptr(opts.seed)
		}

		response, err := client.Image.GenerateImageToImage(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("Image.GenerateImageToImage failed: %w", err)
		}
		return response, nil
	}

	request := minimax.ImageTextToImageRequest{
		Model:           opts.model,
		Prompt:          opts.prompt,
		AspectRatio:     opts.aspectRatio,
		ResponseFormat:  opts.responseFormat,
		N:               intPtr(opts.n),
		PromptOptimizer: boolPtr(opts.promptOptimizer),
		AIGCWatermark:   boolPtr(opts.aigcWatermark),
	}
	if opts.styleType != "" {
		request.Style = &minimax.ImageStyle{StyleType: opts.styleType}
		if opts.styleWeight > 0 {
			request.Style.StyleWeight = floatPtr(opts.styleWeight)
		}
	}
	if opts.width != 0 {
		request.Width = intPtr(opts.width)
		request.Height = intPtr(opts.height)
	}
	if opts.seed != 0 {
		request.Seed = int64Ptr(opts.seed)
	}

	response, err := client.Image.GenerateTextToImage(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("Image.GenerateTextToImage failed: %w", err)
	}
	return response, nil
}

func printResponseSummary(out io.Writer, response *minimax.ImageGenerationResponse) {
	fmt.Fprintf(out, "id=%s\n", response.ID)
	if response.Metadata.SuccessCount != nil || response.Metadata.FailedCount != nil {
		fmt.Fprintf(out, "success_count=%s failed_count=%s\n", formatOptionalInt(response.Metadata.SuccessCount), formatOptionalInt(response.Metadata.FailedCount))
	}
	for index, imageURL := range response.ImageURLs {
		fmt.Fprintf(out, "image_url[%d]=%s\n", index, imageURL)
	}
	for index, imageBase64 := range response.ImageBase64 {
		fmt.Fprintf(out, "image_base64[%d]_bytes=%d\n", index, len(imageBase64))
	}
}

func saveBase64Images(images []string, outputDir string, out io.Writer) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	for index, encoded := range images {
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return fmt.Errorf("decode base64 image %d: %w", index, err)
		}
		path := filepath.Join(outputDir, fmt.Sprintf("image-%02d.png", index+1))
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("write image %d: %w", index, err)
		}
		fmt.Fprintf(out, "saved=%s\n", path)
	}

	return nil
}

func trimOptions(opts *options) {
	opts.apiKey = strings.TrimSpace(opts.apiKey)
	opts.baseURL = strings.TrimSpace(opts.baseURL)
	opts.model = strings.TrimSpace(opts.model)
	opts.prompt = strings.TrimSpace(opts.prompt)
	opts.styleType = strings.TrimSpace(opts.styleType)
	opts.aspectRatio = strings.TrimSpace(opts.aspectRatio)
	opts.responseFormat = strings.TrimSpace(opts.responseFormat)
	opts.outputDir = strings.TrimSpace(opts.outputDir)
}

func formatOptionalInt(value *int) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *value)
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

func envFloatOrDefault(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
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

func envInt64OrDefault(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

type subjectReferenceFlags []minimax.ImageSubjectReference

func (f *subjectReferenceFlags) String() string {
	if f == nil || len(*f) == 0 {
		return ""
	}

	values := make([]string, 0, len(*f))
	for _, reference := range *f {
		values = append(values, reference.Type+"="+reference.ImageFile)
	}
	return strings.Join(values, ",")
}

func (f *subjectReferenceFlags) Set(value string) error {
	reference, err := parseSubjectReference(value)
	if err != nil {
		return err
	}
	*f = append(*f, reference)
	return nil
}

func (f subjectReferenceFlags) ImageSubjectReferences() []minimax.ImageSubjectReference {
	if len(f) == 0 {
		return nil
	}

	references := make([]minimax.ImageSubjectReference, len(f))
	copy(references, f)
	return references
}

func parseSubjectReference(value string) (minimax.ImageSubjectReference, error) {
	value = strings.TrimSpace(value)
	referenceType, imageFile, ok := strings.Cut(value, "=")
	if !ok {
		return minimax.ImageSubjectReference{}, errors.New("subject-reference must use type=image_file")
	}

	referenceType = strings.TrimSpace(referenceType)
	imageFile = strings.TrimSpace(imageFile)
	if referenceType == "" {
		return minimax.ImageSubjectReference{}, errors.New("subject-reference type is empty")
	}
	if imageFile == "" {
		return minimax.ImageSubjectReference{}, errors.New("subject-reference image_file is empty")
	}

	return minimax.ImageSubjectReference{Type: referenceType, ImageFile: imageFile}, nil
}

func boolPtr(value bool) *bool {
	return &value
}

func floatPtr(value float64) *float64 {
	return &value
}

func intPtr(value int) *int {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}
