# Voice Examples

Voice-related examples are organized under `examples/voice/` with five subdirectories:

1. `list/` — list available voices (`Voice.ListVoices`)
2. `design/` — design a custom voice from prompt text (`Voice.DesignVoice`)
3. `clone/` — clone a voice from `audio_url` / `file_id` / local upload (`Voice.CloneVoice`, `Voice.UploadCloneAudio`)
4. `delete/` — delete an owned generated/cloned voice by explicit `voice_id` (`Voice.DeleteVoice`)
5. `upload/` — upload clone or prompt audio directly (`Voice.UploadCloneAudio`, `Voice.UploadPromptAudio`)

## Quick links

- List: `examples/voice/list/README.md`
- Design: `examples/voice/design/README.md`
- Clone: `examples/voice/clone/README.md`
- Upload: `go run ./examples/voice/upload -h`
- Delete: `go run ./examples/voice/delete -h`

## Notes

- The full snapshot of **official non-cloning voices** is maintained in `examples/voice/list/README.md`.
- In China region (`https://api.minimax.chat`), `audio_url` clone may be unsupported; use `file_id` or local upload flow instead.
- Delete examples require an explicit owned `voice_id`; use `voice_generation` for designed voices and `voice_cloning` for cloned voices. Do not pass existing voices that should be kept.
