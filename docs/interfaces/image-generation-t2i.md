# Image Generation T2I

- Official docs: https://platform.minimaxi.com/docs/api-reference/image-generation-t2i.md
- Endpoint: `POST /v1/image_generation`
- SDK status: `Implemented`
- Local code: `image.go`, `image_test.go`, `examples/image/`

## Purpose

Generate images from text prompts.

## Key fields

Request fields include `model`, `prompt`, optional `style`, `aspect_ratio`,
`width`, `height`, `response_format`, `seed`, `n`, `prompt_optimizer`, and
`aigc_watermark`. Response returns image URLs or base64 data plus metadata.

## Development notes

Implemented as `Client.Image.GenerateTextToImage`.

The SDK keeps model names as strings, validates required `model` and `prompt`
client-side, requires custom `width` and `height` to be supplied together,
preserves explicit boolean values through pointer fields, and maps URL/base64
image outputs into a single response type.

Metadata counts are parsed from either JSON numbers or quoted numeric strings
because the official response example shows quoted values.
