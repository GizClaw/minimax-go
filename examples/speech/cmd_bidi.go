package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	minimax "github.com/GizClaw/minimax-go"
)

type bidiOptions struct {
	apiKey, baseURL, model, voiceID, text, textFile, output, report string
	cancelFirst                                                     bool
	cancelAfterAudio                                                bool
	chunkRunes                                                      int
	chunkDelay, timeout                                             time.Duration
}

func bindBidiFlags(fs *flag.FlagSet, opts *bidiOptions) {
	// Do not put a real environment credential in flag defaults/help output.
	fs.StringVar(&opts.apiKey, "api-key", "", "API key; defaults to MINIMAX_API_KEY")
	fs.StringVar(&opts.baseURL, "base-url", envOrDefault("MINIMAX_BASE_URL", exampleDefaultBaseURL), "Provider base URL")
	fs.StringVar(&opts.model, "model", envOrDefault("MINIMAX_SPEECH_MODEL", "speech-2.6-turbo"), "Speech model")
	fs.StringVar(&opts.voiceID, "voice-id", os.Getenv("MINIMAX_SPEECH_VOICE_ID"), "Voice ID")
	fs.StringVar(&opts.text, "text", "Hello from the bidirectional speech example.", "Text to stream")
	fs.StringVar(&opts.textFile, "text-file", "", "Read UTF-8 text from a file instead of -text")
	fs.StringVar(&opts.output, "output", "speech_bidi_output.pcm", "16 kHz mono PCM16LE output")
	fs.StringVar(&opts.report, "report", "", "Optional timing/event JSON output")
	fs.BoolVar(&opts.cancelAfterAudio, "cancel-after-audio", false, "Cancel after receiving the first audio, then resume on the same session")
	fs.BoolVar(&opts.cancelFirst, "cancel-first", false, "Cancel a short prefix, wait for acknowledgement, then stream the full text on the same session")
	fs.IntVar(&opts.chunkRunes, "chunk-runes", 4, "Unicode characters per text event (1..10000)")
	fs.DurationVar(&opts.chunkDelay, "chunk-delay", 80*time.Millisecond, "Simulated input interval; 0 means text is ready")
	fs.DurationVar(&opts.timeout, "timeout", 2*time.Minute, "Whole example deadline")
}

func completeBidiOptions(opts bidiOptions) (bidiOptions, error) {
	if opts.apiKey == "" {
		opts.apiKey = strings.TrimSpace(os.Getenv("MINIMAX_API_KEY"))
	}
	if opts.textFile != "" {
		data, err := os.ReadFile(opts.textFile)
		if err != nil {
			return opts, err
		}
		opts.text = string(data)
	}
	if opts.chunkRunes < 1 || opts.chunkRunes > 10000 || opts.chunkDelay < 0 || opts.timeout <= 0 {
		return opts, errors.New("invalid chunk size, delay or timeout")
	}
	if opts.apiKey == "" || strings.TrimSpace(opts.baseURL) == "" || strings.TrimSpace(opts.model) == "" || strings.TrimSpace(opts.voiceID) == "" || strings.TrimSpace(opts.text) == "" || opts.output == "" {
		return opts, errors.New("API key, base URL, model, voice ID, text and output are required")
	}
	return opts, nil
}

func parseBidiOptions(args []string, out io.Writer) (bidiOptions, error) {
	var opts bidiOptions
	fs := flag.NewFlagSet("bidi", flag.ContinueOnError)
	fs.SetOutput(out)
	bindBidiFlags(fs, &opts)
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	return completeBidiOptions(opts)
}

func runBidiCommand(args []string, stdout, stderr io.Writer) error {
	opts, err := parseBidiOptions(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	client, err := minimax.NewClient(minimax.Config{BaseURL: opts.baseURL, APIKey: opts.apiKey})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()
	report, err := runBidiExample(ctx, client, opts)
	if opts.report != "" {
		if writeErr := writeTimingReport(opts.report, report); writeErr != nil {
			return writeErr
		}
	}
	if err != nil {
		return errors.New(strings.ReplaceAll(err.Error(), opts.apiKey, "[redacted]"))
	}
	return json.NewEncoder(stdout).Encode(report)
}

type speechTimingEvent struct {
	AtMS  float64 `json:"at_ms"`
	Event string  `json:"event"`
	Bytes int     `json:"audio_bytes,omitempty"`
}

type speechTimingReport struct {
	Protocol            string              `json:"protocol"`
	Model               string              `json:"model"`
	VoiceID             string              `json:"voice_id"`
	Status              string              `json:"status"`
	Error               string              `json:"error,omitempty"`
	OpenMS              float64             `json:"open_ms"`
	FirstAudioMS        float64             `json:"first_audio_ms"`
	FirstNonSilentMS    float64             `json:"first_non_silent_ms"`
	CompleteMS          float64             `json:"complete_ms"`
	InputCompleteMS     float64             `json:"input_complete_ms"`
	AudioMS             float64             `json:"audio_ms"`
	AudioBytes          int                 `json:"audio_bytes"`
	InputEvents         int                 `json:"input_events"`
	SentenceStarts      int                 `json:"sentence_starts"`
	SentenceEnds        int                 `json:"sentence_ends"`
	FinalAudioEvents    int                 `json:"final_audio_events"`
	CancelAcknowledged  bool                `json:"cancel_acknowledged"`
	DiscardedAudioBytes int                 `json:"discarded_audio_bytes"`
	FlushAcknowledged   bool                `json:"flush_acknowledged"`
	UsageCharacters     int64               `json:"usage_characters"`
	Events              []speechTimingEvent `json:"events"`
}

func writeTimingReport(path string, report any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Keep whitespace with a nonempty text fragment. The provider discards
// whitespace-only messages, which otherwise loses spaces between LLM tokens.
func bidiTextChunks(text string, size int) []string {
	runes := []rune(text)
	var chunks []string
	pending := ""
	for offset := 0; offset < len(runes); offset += size {
		piece := pending + string(runes[offset:min(offset+size, len(runes))])
		if strings.TrimFunc(piece, unicode.IsSpace) == "" {
			pending = piece
			continue
		}
		pending = ""
		chunks = append(chunks, piece)
	}
	if pending != "" && len(chunks) > 0 {
		chunks[len(chunks)-1] += pending
	}
	var bounded []string
	for _, text := range chunks {
		runes := []rune(text)
		for offset := 0; offset < len(runes); offset += 10000 {
			bounded = append(bounded, string(runes[offset:min(offset+10000, len(runes))]))
		}
	}
	return bounded
}

func runBidiExample(ctx context.Context, client *minimax.Client, opts bidiOptions) (report speechTimingReport, resultErr error) {
	started := time.Now()
	report = speechTimingReport{Protocol: "bidi", Model: opts.model, VoiceID: opts.voiceID, Status: "running", FirstAudioMS: -1, FirstNonSilentMS: -1}
	at := func() float64 { return float64(time.Since(started)) / float64(time.Millisecond) }
	defer func() {
		report.CompleteMS = at()
		if resultErr != nil {
			report.Status = "error"
			report.Error = strings.ReplaceAll(resultErr.Error(), opts.apiKey, "[redacted]")
		} else {
			report.Status = "complete"
		}
	}()
	if err := os.MkdirAll(filepath.Dir(opts.output), 0700); err != nil {
		return report, err
	}
	file, err := os.OpenFile(opts.output, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return report, err
	}
	defer closeWithRetError(file, "audio output", &resultErr)
	rate, channels := 16000, 1
	session, err := client.Speech.OpenBidi(ctx, minimax.SpeechBidiRequest{
		Model: opts.model, VoiceID: opts.voiceID,
		AudioSetting: &minimax.SpeechAudioSetting{Format: "pcm", SampleRate: &rate, Channel: &channels},
	})
	if err != nil {
		return report, err
	}
	defer closeWithRetError(session, "bidi session", &resultErr)
	report.OpenMS = at()
	if opts.cancelFirst || opts.cancelAfterAudio {
		runes := []rune(opts.text)
		prefix := string(runes[:min(5, len(runes))])
		if opts.cancelAfterAudio {
			prefix = opts.text
		}
		if err := session.SendText(ctx, prefix); err != nil {
			return report, err
		}
		if opts.cancelAfterAudio {
			for {
				event, err := session.Next(ctx)
				if err != nil {
					return report, err
				}
				report.DiscardedAudioBytes += len(event.Audio)
				if len(event.Audio) > 0 {
					break
				}
				if event.Done {
					return report, errors.New("session ended before active cancellation")
				}
			}
		}
		if err := session.Cancel(ctx); err != nil {
			return report, err
		}
		for {
			event, err := session.Next(ctx)
			if err != nil {
				return report, err
			}
			report.DiscardedAudioBytes += len(event.Audio)
			if event.Event == "task_canceled" {
				report.CancelAcknowledged = true
				break
			}
			if event.Done {
				return report, errors.New("cancel unexpectedly closed the session")
			}
		}
	}
	if err := streamBidiTurn(ctx, session, opts, started, file, &report); err != nil {
		return report, err
	}
	if err := session.Finish(ctx); err != nil {
		return report, err
	}
	for {
		event, err := session.Next(ctx)
		if err != nil {
			return report, err
		}
		if event.Done {
			return report, nil
		}
		if len(event.Audio) > 0 {
			return report, errors.New("received audio after task_flushed")
		}
	}
}

func streamBidiTurn(ctx context.Context, session *minimax.SpeechBidi, opts bidiOptions, started time.Time, file io.Writer, report *speechTimingReport) (resultErr error) {
	at := func() float64 { return float64(time.Since(started)) / float64(time.Millisecond) }
	ctx, cancel := context.WithCancel(ctx)
	sent := make(chan struct{})
	var sendErr error
	var inputMS float64
	chunks := bidiTextChunks(opts.text, opts.chunkRunes)
	report.InputEvents = len(chunks)
	go func() {
		defer func() {
			if sendErr != nil {
				cancel()
			}
			close(sent)
		}()
		for index, text := range chunks {
			timer := time.NewTimer(max(0, time.Until(started.Add(time.Duration(index)*opts.chunkDelay))))
			select {
			case <-ctx.Done():
				timer.Stop()
				sendErr = context.Cause(ctx)
				return
			case <-timer.C:
			}
			if sendErr = session.SendText(ctx, text); sendErr != nil {
				return
			}
		}
		inputMS = at()
		sendErr = session.Flush(ctx)
	}()
	defer func() {
		cancel()
		if resultErr != nil {
			_ = session.Close()
		}
		<-sent
		report.InputCompleteMS = inputMS
		if resultErr == nil {
			resultErr = sendErr
		}
	}()
	for {
		event, err := session.Next(ctx)
		if err != nil {
			return err
		}
		elapsed := at()
		report.Events = append(report.Events, speechTimingEvent{AtMS: elapsed, Event: event.Event, Bytes: len(event.Audio)})
		if event.UsageCharacters != nil {
			report.UsageCharacters += *event.UsageCharacters
		}
		if event.IsFinal {
			report.FinalAudioEvents++
		}
		switch event.Event {
		case "sentence_start":
			report.SentenceStarts++
		case "sentence_end":
			report.SentenceEnds++
		case "task_flushed":
			<-sent
			if sendErr != nil {
				return sendErr
			}
			report.FlushAcknowledged = true
			if report.AudioBytes == 0 {
				return errors.New("bidi flush completed without audio")
			}
			return nil
		}
		if len(event.Audio) > 0 {
			if report.FirstAudioMS < 0 {
				report.FirstAudioMS = elapsed
			}
			if report.FirstNonSilentMS < 0 && pcmHasVoice(event.Audio) {
				report.FirstNonSilentMS = elapsed
			}
			if _, err := file.Write(event.Audio); err != nil {
				return err
			}
			report.AudioBytes += len(event.Audio)
			report.AudioMS = float64(report.AudioBytes) / 32
		}
		if event.Done {
			if !report.FlushAcknowledged || report.AudioBytes == 0 {
				return errors.New("bidi session ended without flush acknowledgement or audio")
			}
			return nil
		}
	}
}

func pcmHasVoice(data []byte) bool {
	for offset := 0; offset+1 < len(data); offset += 2 {
		value := int16(uint16(data[offset]) | uint16(data[offset+1])<<8)
		if value > 184 || value < -184 {
			return true
		}
	}
	return false
}
