package minimax

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"testing"
	"time"
)

func TestSpeechHTTPFinalFrameConnectionReuseWithChunkedTail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"data\":{\"status\":2}}\n\n")
		w.(http.Flusher).Flush()
		// Terminal application data arrives before the HTTP body EOF.
		select {
		case <-time.After(10 * time.Millisecond):
			fmt.Fprint(w, ": trailing keepalive\n\n")
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	for index := range 3 {
		reused := false
		ctx := httptrace.WithClientTrace(t.Context(), &httptrace.ClientTrace{
			GotConn: func(info httptrace.GotConnInfo) { reused = info.Reused },
		})
		stream, err := client.Speech.OpenStream(ctx, SpeechStreamRequest{Text: "hello"})
		if err != nil {
			t.Fatal(err)
		}
		event, err := stream.Next()
		if err != nil || !event.Done {
			t.Fatalf("terminal: %+v %v", event, err)
		}
		if err := stream.Close(); err != nil {
			t.Fatal(err)
		}
		if reused != (index > 0) {
			t.Fatalf("request %d reused=%t", index, reused)
		}
	}
}

func TestSpeechHTTPFinalCloseBoundsTailWait(t *testing.T) {
	cancelled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"data\":{\"status\":2}}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(cancelled)
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := client.Speech.OpenStream(t.Context(), SpeechStreamRequest{Text: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Next(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- stream.Close() }()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("terminal drain kept Close blocked")
	}
	select {
	case <-cancelled:
	case <-ctx.Done():
		t.Fatal("terminal drain did not cancel an open-ended response")
	}
}
