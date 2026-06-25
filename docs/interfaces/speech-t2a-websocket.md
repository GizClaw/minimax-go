# Speech T2A WebSocket

- Official docs: https://platform.minimaxi.com/docs/api-reference/speech-t2a-websocket.md
- Endpoint: `t2a_v2_websocket`
- SDK status: `Not implemented`
- Local code: `speech_stream.go` implements an HTTP stream/SSE helper, not an
  official WebSocket client.

## Purpose

Synchronous streaming speech synthesis through MiniMax's WebSocket protocol.

## Development notes

Implement as a separate client path. Do not overload `Speech.OpenStream` unless
the wire protocol is intentionally migrated. Tests should cover handshake,
message framing, server error frames, context cancellation, and close behavior.

