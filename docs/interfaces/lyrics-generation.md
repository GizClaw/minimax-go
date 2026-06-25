# Lyrics Generation

- Official docs: https://platform.minimaxi.com/docs/api-reference/lyrics-generation.md
- Endpoint: `POST /v1/lyrics_generation`
- SDK status: `Implemented`
- Local code: `Music.GenerateLyrics` in `music.go`; tests in `music_test.go`; example in `examples/music`.

## Purpose

Generate or edit lyrics for music generation workflows.

## Development notes

Add under a future `MusicService`. Keep generated lyric text and any structured
sections separate from raw payload fields.

The SDK supports `write_full_song` and `edit` modes. `edit` mode requires
non-empty existing lyrics before the request is sent.
