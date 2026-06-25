# Image Generation T2I

- Official docs: https://platform.minimaxi.com/docs/api-reference/image-generation-t2i.md
- Endpoint: `POST /v1/image_generation`
- SDK status: `Not implemented`
- Local code: none.

## Purpose

Generate images from text prompts.

## Key fields

Request fields include `model`, `prompt`, optional `style`, `aspect_ratio`,
`width`, `height`, `response_format`, `seed`, `n`, `prompt_optimizer`, and
`aigc_watermark`. Response returns image URLs or base64 data plus metadata.

## Development notes

Add an `ImageService` with text and image variants sharing request/response
types where possible.

