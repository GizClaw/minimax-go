# MiniMax music example

`examples/music` demonstrates lyrics generation, direct music generation, cover
preprocessing, and cover generation.

## Quick start

```bash
export MINIMAX_API_KEY="your_api_key"

go run ./examples/music generate \
  -model music-2.6-free \
  -prompt "bright chiptune, playful, short game theme" \
  -lyrics "[Verse]\nTiny lights wake up the room\n[Chorus]\nBuild it bright and let it play" \
  -output-format url
```

## Generate lyrics

```bash
go run ./examples/music lyrics \
  -lyrics-mode write_full_song \
  -prompt "a bright summer beach pop song"
```

Use `-lyrics-mode edit` with `-lyrics` or `-lyrics-file` to edit existing lyrics.

## Instrumental music

```bash
go run ./examples/music generate \
  -instrumental \
  -prompt "cinematic ambient synthwave, slow build" \
  -output-format url
```

## Cover preprocess

```bash
go run ./examples/music preprocess \
  -audio-url https://example.com/original-song.mp3
```

The command prints `cover_feature_id`, which can be passed to `cover` mode.

## Cover generation

One-step cover:

```bash
go run ./examples/music cover \
  -audio-url https://example.com/original-song.mp3 \
  -prompt "jazz lounge, late night saxophone" \
  -output-format url
```

Two-step cover:

```bash
go run ./examples/music cover \
  -cover-feature-id FEATURE_ID \
  -lyrics-file edited-lyrics.txt \
  -prompt "jazz lounge, late night saxophone" \
  -output-format url
```

## Save returned audio

`-output` saves returned audio locally. If the response is a URL, the example
downloads it. If the response is hex audio, the example decodes it.

```bash
go run ./examples/music generate \
  -output-format url \
  -output /tmp/minimax-music.mp3
```

## Environment variables

- `MINIMAX_API_KEY`
- `MINIMAX_BASE_URL`
- `MINIMAX_MUSIC_MODEL`
- `MINIMAX_MUSIC_LYRICS_MODE`
- `MINIMAX_MUSIC_PROMPT`
- `MINIMAX_MUSIC_LYRICS`
- `MINIMAX_MUSIC_LYRICS_FILE`
- `MINIMAX_MUSIC_TITLE`
- `MINIMAX_MUSIC_OUTPUT_FORMAT`
- `MINIMAX_MUSIC_SAMPLE_RATE`
- `MINIMAX_MUSIC_BITRATE`
- `MINIMAX_MUSIC_AUDIO_FORMAT`
- `MINIMAX_MUSIC_AIGC_WATERMARK`
- `MINIMAX_MUSIC_LYRICS_OPTIMIZER`
- `MINIMAX_MUSIC_INSTRUMENTAL`
- `MINIMAX_MUSIC_AUDIO_URL`
- `MINIMAX_MUSIC_AUDIO_BASE64`
- `MINIMAX_MUSIC_COVER_FEATURE_ID`
- `MINIMAX_MUSIC_OUTPUT`
- `MINIMAX_MUSIC_TIMEOUT`
