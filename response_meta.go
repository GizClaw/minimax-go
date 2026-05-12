package minimax

import (
	"net/http"

	"github.com/GizClaw/minimax-go/internal/transport"
)

// ResponseMeta contains transport-level metadata returned by Minimax APIs.
type ResponseMeta struct {
	RequestID  string      `json:"request_id,omitempty"`
	TraceID    string      `json:"trace_id,omitempty"`
	HTTPStatus int         `json:"http_status,omitempty"`
	Header     http.Header `json:"-"`
}

func responseMetaFromTransport(meta transport.ResponseMeta) ResponseMeta {
	return ResponseMeta{
		RequestID:  meta.RequestID,
		TraceID:    meta.TraceID,
		HTTPStatus: meta.HTTPStatus,
		Header:     meta.Header.Clone(),
	}
}
