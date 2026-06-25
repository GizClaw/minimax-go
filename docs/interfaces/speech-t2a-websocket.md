# Speech T2A WebSocket

- Official docs: https://platform.minimaxi.com/docs/api-reference/speech-t2a-websocket.md
- Endpoint: `WSS /ws/v1/t2a_v2`
- SDK status: `Implemented`
- Local code: `Speech.OpenWebSocket` in `speech_websocket.go`; tests in
  `speech_websocket_test.go`.

## Purpose

Synchronous streaming speech synthesis through MiniMax's WebSocket protocol.

## Current SDK shape

The SDK keeps this as a separate path from `Speech.OpenStream`. It dials the
official WebSocket endpoint, waits for `connected_success`, sends `task_start`,
waits for `task_started`, sends `task_continue` and `task_finish`, then reads
audio and terminal events with contextual errors for server failure frames.
