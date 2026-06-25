# Responses Create

- Official docs: https://platform.minimaxi.com/docs/api-reference/responses-create.md
- Endpoint: `POST /v1/responses`
- SDK status: `Not implemented`
- Local code: none.

## Purpose

OpenAI Responses API compatible model invocation.

## Development notes

Decide whether this SDK should wrap text compatibility APIs or leave callers to
official OpenAI/Anthropic SDKs. If implemented, keep it in a `TextService` and
avoid coupling it to multimodal generation services.

