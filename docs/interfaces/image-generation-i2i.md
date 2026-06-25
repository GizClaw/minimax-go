# Image Generation I2I

- Official docs: https://platform.minimaxi.com/docs/api-reference/image-generation-i2i.md
- Endpoint: `POST /v1/image_generation`
- SDK status: `Implemented`
- Local code: `image.go`, `image_test.go`, `examples/image/`

## Purpose

Generate images from text plus reference image input.

## Development notes

Implemented as `Client.Image.GenerateImageToImage`.

The SDK reuses the existing image generation response mapping and transport
behavior. Subject references are modeled as `ImageSubjectReference` values and
sent through the official `subject_reference` wire field with `type` and
`image_file`.

The method validates required `model`, `prompt`, and subject reference fields
client-side, then reuses the same dimension, `n`, style, and response-format
validation semantics as text-to-image.
