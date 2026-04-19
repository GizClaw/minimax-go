package minimax

import (
	"net/http"

	"github.com/GizClaw/minimax-go/internal/transport"
)

type Config struct {
	BaseURL        string
	APIKey         string
	HTTPClient     *http.Client
	DefaultHeaders http.Header
	Retry          transport.RetryConfig
}

type Client struct {
	transport   *transport.Client
	Speech      *SpeechService
	SpeechAsync *SpeechAsyncService
	File        *FileService
	Voice       *VoiceService
}

func NewClient(config Config) (*Client, error) {
	trans, err := transport.New(transport.Config{
		BaseURL:        config.BaseURL,
		APIKey:         config.APIKey,
		HTTPClient:     config.HTTPClient,
		DefaultHeaders: config.DefaultHeaders,
		Retry:          config.Retry,
	})
	if err != nil {
		return nil, err
	}

	client := &Client{transport: trans}
	client.Speech = &SpeechService{
		transport:      trans,
		endpoint:       defaultSpeechSynthesizePath,
		streamEndpoint: defaultSpeechStreamPath,
	}
	client.SpeechAsync = &SpeechAsyncService{
		transport:      trans,
		submitEndpoint: defaultSpeechAsyncSubmitPath,
		queryEndpoint:  defaultSpeechAsyncQueryPath,
	}
	client.File = &FileService{
		transport:      trans,
		uploadEndpoint: defaultFileUploadPath,
		maxUploadBytes: defaultFileMaxUploadBytes,
	}
	client.Voice = &VoiceService{
		transport:      trans,
		endpoint:       defaultVoiceListPath,
		designEndpoint: defaultVoiceDesignPath,
		cloneEndpoint:  defaultVoiceClonePath,
	}

	return client, nil
}
