# Ark Responses API examples

Run from `components/model/ark/v2`. Set `ARK_API_KEY` and `ARK_MODEL` in your
process environment; `ARK_BASE_URL` is optional. These examples make real
inference calls. The default deadline for the entire conversation is 3 minutes.

```sh
go run ./examples/responses -help
go run ./examples/responses -mode generate -prompt "Reply EXAMPLE_OK." -expect EXAMPLE_OK
```

`-expect` checks the final answer and returns a nonzero exit status when the
expected text is missing. The example also validates tool arguments/results,
structured JSON, reasoning passback, cache recall, and modifier execution in the
corresponding modes. It prints response IDs, finish reasons, and usage.

## Text, tools, reasoning, and modifiers

```sh
go run ./examples/responses -mode stream -prompt "Reply STREAM_OK." -expect STREAM_OK
go run ./examples/responses -mode tools -expect 21
go run ./examples/responses -mode structured -prompt "Return answer as the string 21." -expect 21
go run ./examples/responses -mode reasoning -expect 324
go run ./examples/responses -mode modifiers -prompt "Reply MODIFIER_OK." -expect MODIFIER_OK
```

| Mode | Behavior |
| --- | --- |
| `generate` | Generate text and print response metadata |
| `stream` | Display deltas immediately and concatenate messages with final usage |
| `tools` | Stream and assemble an `add(8, 13)` call, execute it locally, and send its result back for a final answer |
| `structured` | Request strict JSON schema output and decode the `answer` field |
| `reasoning` | Stream the reasoning for `17 × 19`, preserve its assistant message, then pass it back for an `add one` follow-up |
| `modifiers` | Edit request JSON, inspect original response JSON, count original SSE events, and verify exactly one successful finalization |

Thinking defaults to disabled, except in `reasoning` mode. `-thinking enabled`,
`disabled`, or `auto` overrides this setting. The reasoning example requires a
model that produces reasoning content; its follow-up uses thinking disabled.

## Multimodal input and file lifecycle

The bundled [marker PDF](testdata/marker.pdf) contains the text `PDF_OK_731`.
`-kind` accepts `image`, `audio`, `video`, or `file`. Select the input using
`-source url|base64|file_id|upload` and `-value`:

```sh
go run ./examples/responses -mode multimodal -kind image -source base64 -value ../examples/generate_with_image/eino.png -prompt "Describe this image briefly."
go run ./examples/responses -mode multimodal -kind image -source upload -value ../examples/generate_with_image/eino.png -prompt "Describe this image briefly."
go run ./examples/responses -mode multimodal -kind file -source base64 -value ./examples/responses/testdata/marker.pdf -prompt "Read the verification marker." -expect PDF_OK_731
go run ./examples/responses -mode multimodal -kind file -source upload -value ./examples/responses/testdata/marker.pdf -prompt "Read the verification marker." -expect PDF_OK_731
go run ./examples/responses -mode multimodal -kind video -source url -value "$ARK_VIDEO_URL" -prompt "Summarize this video."
go run ./examples/responses -mode multimodal -kind audio -source base64 -value /path/to/audio.mp3 -mime audio/mpeg -prompt "Transcribe this audio."
go run ./examples/responses -mode stream -file-id "$ARK_FILE_ID" -prompt "Summarize this document."
```

- `base64` reads a local file. MIME type is inferred from its extension or set
  explicitly with `-mime`.
- `upload` uploads a local file through the SDK Files API, waits for `active`,
  uses the returned FileID for inference, and deletes that exact upload on both
  success and failure. Cleanup uses its own deadline and reports errors.
- `file_id` and `-file-id` use an existing processed file; the example does not
  delete it. `-file-id ID` is shorthand for a document input.
- `url` sends a reachable URL directly. Each media kind requires a capable model.

Return media from a function tool using the same source options:

```sh
go run ./examples/responses -mode tool-result -kind file -source upload -value ./examples/responses/testdata/marker.pdf -prompt "Read the exact verification marker." -expect PDF_OK_731
```

This executes `read_media`, returns a native multimodal tool result with the
matching call ID, and asks the model for its answer. Request acceptance does not
establish that a model read the media correctly; use `-expect` with a known
fixture to check the answer.

## Cache and built-in search

```sh
go run ./examples/responses -mode session -expect CACHE_731
go run ./examples/responses -mode prefix -expect PREFIX_731
go run ./examples/responses -mode web-search -prompt "Search for the official CloudWeGo Eino GitHub repository." -expect github.com/cloudwego/eino
```

`session` stores a first response and reuses its cached history. `prefix` creates
a prefix and continues it using `HeadPreviousResponseID`. These modes use fixed
codes to verify recall. They require account/model cache availability.
`web-search` requires the account's built-in search service to be activated.
Service errors retain a nonzero exit status.

See [VALIDATION.md](VALIDATION.md) for runs of this example against the supplied
model, including permission and content-validation failures.
