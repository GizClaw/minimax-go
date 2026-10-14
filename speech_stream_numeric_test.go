package minimax

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSpeechStreamOfficialNumericStatusAndUsage(t *testing.T) {
	for _, aggregate := range []bool{false, true} {
		t.Run(fmt.Sprint(aggregate), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					Model         string               `json:"model"`
					Stream        bool                 `json:"stream"`
					StreamOptions *SpeechStreamOptions `json:"stream_options"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				if !request.Stream || request.Model != "speech-2.6-turbo" || request.StreamOptions == nil || request.StreamOptions.ExcludeAggregatedAudio == aggregate {
					t.Errorf("request = %+v", request)
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"data\":{\"audio\":\"0102\",\"status\":1}}\n\n")
				last := ""
				if aggregate {
					last = "0102"
				}
				fmt.Fprintf(w, "data: {\"data\":{\"audio\":%q,\"status\":2},\"extra_info\":{\"usage_characters\":3}}\n\n", last)
			}))
			defer server.Close()
			client, err := NewClient(Config{BaseURL: server.URL, HTTPClient: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			stream, err := client.Speech.OpenStream(context.Background(), SpeechStreamRequest{Model: "speech-2.6-turbo", Text: "好，", VoiceID: "voice", StreamOptions: &SpeechStreamOptions{ExcludeAggregatedAudio: !aggregate}})
			if err != nil {
				t.Fatal(err)
			}
			defer stream.Close()
			first, err := stream.Next()
			if err != nil || first.Done || string(first.Audio) != "\x01\x02" {
				t.Fatalf("delta = %+v, err=%v", first, err)
			}
			last, err := stream.Next()
			if err != nil || !last.Done || len(last.Audio) != 0 || last.UsageCharacters == nil || *last.UsageCharacters != 3 {
				t.Fatalf("terminal = %+v, err=%v", last, err)
			}
			if aggregate && string(last.AggregatedAudio) != "\x01\x02" || !aggregate && len(last.AggregatedAudio) != 0 {
				t.Fatalf("aggregate = %v", last.AggregatedAudio)
			}
			if _, err := stream.Next(); err != io.EOF {
				t.Fatalf("after terminal: %v", err)
			}
		})
	}
}

func TestSpeechStreamStatusRejectsInvalidTypes(t *testing.T) {
	for _, status := range []string{"true", "{}", "1.5"} {
		var value speechStreamStatus
		if err := json.Unmarshal([]byte(status), &value); err == nil {
			t.Fatalf("accepted status %s", status)
		}
	}
}
