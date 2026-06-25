# Responses Input Tokens

- Official docs: https://platform.minimaxi.com/docs/api-reference/responses-input-tokens.md
- Endpoint: `POST /v1/responses/input_tokens`
- SDK status: `Not implemented`
- Local code: none.

## Purpose

Estimate input token usage for a Responses request without generating output.

## Development notes

Implement only if `Responses.Create` is in scope. Reuse the same input schema to
avoid divergence.

