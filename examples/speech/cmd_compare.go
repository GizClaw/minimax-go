package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"strings"
	"time"

	minimax "github.com/GizClaw/minimax-go"
)

type speechComparisonRun struct {
	Iteration        int                `json:"iteration"`
	ConnectionReused bool               `json:"connection_reused"`
	Timing           speechTimingReport `json:"timing"`
}

type speechComparison struct {
	Model      string                `json:"model"`
	VoiceID    string                `json:"voice_id"`
	Text       string                `json:"text"`
	ChunkRunes int                   `json:"chunk_runes"`
	ChunkDelay string                `json:"chunk_delay"`
	Scope      string                `json:"scope"`
	Runs       []speechComparisonRun `json:"runs"`
}

func runSpeechComparisonCommand(args []string, stdout, stderr io.Writer) (resultErr error) {
	var opts bidiOptions
	var iterations int
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(stderr)
	bindBidiFlags(fs, &opts)
	_ = fs.Set("output", "speech_compare")
	fs.Lookup("output").DefValue = "speech_compare"
	fs.IntVar(&iterations, "iterations", 3, "Repeated turns per protocol (1..20)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	var err error
	opts, err = completeBidiOptions(opts)
	if err != nil {
		return err
	}
	if opts.cancelFirst || opts.cancelAfterAudio {
		return errors.New("cancel-first is a bidi-only demonstration; omit it for a timing comparison")
	}
	if iterations < 1 || iterations > 20 {
		return errors.New("iterations must be in 1..20")
	}
	if err := os.MkdirAll(opts.output, 0700); err != nil {
		return err
	}
	reportPath := opts.report
	if reportPath == "" {
		reportPath = filepath.Join(opts.output, "comparison.json")
	}
	comparison := speechComparison{
		Model: opts.model, VoiceID: opts.voiceID, Text: opts.text,
		ChunkRunes: opts.chunkRunes, ChunkDelay: opts.chunkDelay.String(),
		Scope: "Raw SDK examples, PCM16LE 16kHz mono. HTTP/ordinary WS submit complete text; bidi sends chunks as available. With delay>0, HTTP/WS wait for the entire simulated input. This is not GizClaw's client-batching pipeline or device playback. Each protocol has an isolated transport pool: HTTP reuses its pool; ordinary WS opens each time; bidi reuses one session after flush. First audio/non-silent metrics are receive times, not speaker output.",
	}
	defer func() {
		if err := writeTimingReport(reportPath, comparison); resultErr == nil {
			resultErr = err
		}
		if resultErr != nil {
			resultErr = errors.New(strings.ReplaceAll(resultErr.Error(), opts.apiKey, "[redacted]"))
		}
	}()
	baseTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return errors.New("compare requires an http.Transport default for isolated pools")
	}
	clients := make(map[string]*minimax.Client)
	for _, protocol := range []string{"http", "websocket", "bidi"} {
		transport := baseTransport.Clone()
		defer transport.CloseIdleConnections()
		client, err := minimax.NewClient(minimax.Config{BaseURL: opts.baseURL, APIKey: opts.apiKey, HTTPClient: &http.Client{Transport: transport}})
		if err != nil {
			return err
		}
		clients[protocol] = client
	}
	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout*time.Duration(iterations*3))
	defer cancel()
	var bidi *minimax.SpeechBidi
	defer func() {
		if bidi != nil {
			_ = bidi.Close()
		}
	}()
	for iteration := 1; iteration <= iterations; iteration++ {
		protocols := []string{"http", "websocket", "bidi"}
		// Rotate protocol order while retaining each protocol's connection state.
		if iteration%2 == 0 {
			protocols = []string{"bidi", "websocket", "http"}
		}
		for _, protocol := range protocols {
			client := clients[protocol]
			current := opts
			current.output = filepath.Join(opts.output, fmt.Sprintf("%s-%d.pcm", protocol, iteration))
			turnCtx, turnCancel := context.WithTimeout(ctx, opts.timeout)
			started := time.Now()
			timing := speechTimingReport{Protocol: protocol, Model: opts.model, VoiceID: opts.voiceID, Status: "running", FirstAudioMS: -1, FirstNonSilentMS: -1}
			reused := false
			file, err := os.OpenFile(current.output, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			if err == nil {
				if protocol == "bidi" {
					if bidi == nil {
						rate, channel := 16000, 1
						bidi, err = client.Speech.OpenBidi(turnCtx, minimax.SpeechBidiRequest{Model: opts.model, VoiceID: opts.voiceID,
							AudioSetting: &minimax.SpeechAudioSetting{Format: "pcm", SampleRate: &rate, Channel: &channel}})
						timing.OpenMS = elapsedMS(started)
					} else {
						reused = true
					}
					if err == nil {
						err = streamBidiTurn(turnCtx, bidi, current, started, file, &timing)
					}
				} else {
					turnCtx = httptrace.WithClientTrace(turnCtx, &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) { reused = info.Reused }})
					err = runCompleteTextComparison(turnCtx, client, current, protocol, started, file, &timing)
				}
				if closeErr := file.Close(); err == nil {
					err = closeErr
				}
			}
			turnCancel()
			timing.CompleteMS = elapsedMS(started)
			timing.Status = "complete"
			if err != nil {
				timing.Status, timing.Error = "error", strings.ReplaceAll(err.Error(), opts.apiKey, "[redacted]")
			}
			comparison.Runs = append(comparison.Runs, speechComparisonRun{Iteration: iteration, ConnectionReused: reused, Timing: timing})
			if err := json.NewEncoder(stdout).Encode(comparison.Runs[len(comparison.Runs)-1]); err != nil {
				return err
			}
			if err != nil {
				return err
			}
		}
	}
	if err := bidi.Finish(ctx); err != nil {
		return err
	}
	for {
		event, err := bidi.Next(ctx)
		if err != nil {
			return err
		}
		if len(event.Audio) > 0 {
			return errors.New("received audio after the final flush")
		}
		if event.Done {
			return nil
		}
	}
}

func elapsedMS(start time.Time) float64 {
	return float64(time.Since(start)) / float64(time.Millisecond)
}

func runCompleteTextComparison(ctx context.Context, client *minimax.Client, opts bidiOptions, protocol string, started time.Time, file io.Writer, report *speechTimingReport) error {
	ready := started.Add(time.Duration(len(bidiTextChunks(opts.text, opts.chunkRunes))-1) * opts.chunkDelay)
	timer := time.NewTimer(max(0, time.Until(ready)))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-timer.C:
	}
	report.InputCompleteMS = elapsedMS(started)
	report.InputEvents = 1
	rate, channel := 16000, 1
	audio := &minimax.SpeechAudioSetting{Format: "pcm", SampleRate: &rate, Channel: &channel}
	var next func() ([]byte, *int64, bool, error)
	var closeStream func() error
	openStart := time.Now()
	if protocol == "http" {
		stream, err := client.Speech.OpenStream(ctx, minimax.SpeechStreamRequest{
			Model: opts.model, VoiceID: opts.voiceID, Text: opts.text, AudioSetting: audio,
			StreamOptions: &minimax.SpeechStreamOptions{ExcludeAggregatedAudio: true}})
		if err != nil {
			return err
		}
		closeStream = stream.Close
		next = func() ([]byte, *int64, bool, error) {
			event, err := stream.Next()
			if err != nil {
				return nil, nil, false, err
			}
			return event.Audio, event.UsageCharacters, event.Done, nil
		}
	} else {
		stream, err := client.Speech.OpenWebSocket(ctx, minimax.SpeechWebSocketRequest{Model: opts.model, VoiceID: opts.voiceID, Text: opts.text, AudioSetting: audio})
		if err != nil {
			return err
		}
		closeStream = stream.Close
		next = func() ([]byte, *int64, bool, error) {
			event, err := stream.Next(ctx)
			if err != nil {
				return nil, nil, false, err
			}
			return event.Audio, event.UsageCharacters, event.Done, nil
		}
	}
	report.OpenMS = elapsedMS(openStart)
	defer closeStream()
	for {
		audio, usage, done, err := next()
		if err != nil {
			return err
		}
		if usage != nil {
			report.UsageCharacters += *usage
		}
		if len(audio) > 0 {
			at := elapsedMS(started)
			report.Events = append(report.Events, speechTimingEvent{AtMS: at, Event: "audio", Bytes: len(audio)})
			if report.FirstAudioMS < 0 {
				report.FirstAudioMS = at
			}
			if report.FirstNonSilentMS < 0 && pcmHasVoice(audio) {
				report.FirstNonSilentMS = at
			}
			if _, err := file.Write(audio); err != nil {
				return err
			}
			report.AudioBytes += len(audio)
			report.AudioMS = float64(report.AudioBytes) / 32
		}
		if done {
			if report.AudioBytes == 0 {
				return errors.New("synthesis completed without audio")
			}
			return nil
		}
	}
}
