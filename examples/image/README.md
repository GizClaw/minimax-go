# MiniMax image generation example

Run a text-to-image request:

```bash
export MINIMAX_API_KEY="your_api_key"
go run ./examples/image \
  -model image-01 \
  -prompt "A tiny desktop robot drawing a green circuit board" \
  -aspect-ratio 1:1 \
  -response-format url
```

Run an image-to-image request:

```bash
go run ./examples/image \
  -model image-01 \
  -prompt "A girl looking into the distance from a library window" \
  -aspect-ratio 16:9 \
  -subject-reference character=https://example.com/reference.png
```

Save base64 image outputs:

```bash
go run ./examples/image \
  -response-format base64 \
  -output-dir ./tmp-images
```
