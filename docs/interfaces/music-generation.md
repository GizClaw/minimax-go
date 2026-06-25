# Music Generation

- Official docs: https://platform.minimaxi.com/docs/api-reference/music-generation.md
- Endpoint: `POST /v1/music_generation`
- SDK status: `Not implemented`
- Local code: none.

## Purpose

Generate music from a song description and lyrics, or generate cover music from
reference audio/features.

## Key fields

Models include `music-2.6`, `music-cover`, `music-2.6-free`, and
`music-cover-free`. Request fields include `prompt`, `lyrics`, `stream`,
`output_format`, `audio_setting`, `aigc_watermark`, `lyrics_optimizer`,
`is_instrumental`, `audio_url`, `audio_base64`, and `cover_feature_id`.

## Development notes

Start non-streaming first. Add stream support only after the shared stream
parsing behavior is clear for music events.

