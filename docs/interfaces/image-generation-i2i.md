# Image Generation I2I

- Official docs: https://platform.minimaxi.com/docs/api-reference/image-generation-i2i.md
- Endpoint: `POST /v1/image_generation`
- SDK status: `Not implemented`
- Local code: none.

## Purpose

Generate images from text plus reference image input.

## Development notes

Implement with the same `ImageService` used by text-to-image. Model image
inputs explicitly rather than using `map[string]any` for stable fields, and
reuse the existing `ImageGenerationResponse` mapping when the response shape
matches T2I.

This remains a planned API after the T2I implementation.
