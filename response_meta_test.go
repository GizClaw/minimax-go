package minimax

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResponseMetaJSON(t *testing.T) {
	t.Parallel()

	t.Run("zero response meta is omitted", func(t *testing.T) {
		t.Parallel()

		data, err := json.Marshal(SpeechAsyncSubmitResponse{TaskID: "task-1"})
		if err != nil {
			t.Fatalf("Marshal() error = %v, want nil", err)
		}

		if strings.Contains(string(data), "response_meta") {
			t.Fatalf("Marshal() = %s, want response_meta omitted", data)
		}
	})

	t.Run("non-zero response meta is included", func(t *testing.T) {
		t.Parallel()

		data, err := json.Marshal(SpeechAsyncSubmitResponse{
			ResponseMeta: ResponseMeta{TraceID: "trace-1"},
			TaskID:       "task-1",
		})
		if err != nil {
			t.Fatalf("Marshal() error = %v, want nil", err)
		}

		if !strings.Contains(string(data), `"trace_id":"trace-1"`) {
			t.Fatalf("Marshal() = %s, want trace_id", data)
		}
	})
}
