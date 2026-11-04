# Ark Responses API code examples

Each directory is a small, standalone Go example for one API feature. Read its
`main.go` to see the model construction, message types, and calls directly. Shared
[configuration](../internal/exampleutil/responses.go) only reads credentials and
connection settings from the environment, applies a three-minute deadline, and
prints common response metadata.

Run from `components/model/ark/v2`. Set `ARK_API_KEY` and `ARK_MODEL` in your
environment; `ARK_BASE_URL` is optional. There are no command-line options or
mode dispatcher. For example:

```sh
go run ./examples/responses/generate
go run ./examples/responses/stream
go run ./examples/responses/tools
```

| Example | Code to read | Environment values |
| --- | --- | --- |
| [generate](generate/main.go) | Generate a text response | Optional `ARK_PROMPT` |
| [stream](stream/main.go) | Receive deltas and concatenate final usage | Optional `ARK_PROMPT` |
| [tools](tools/main.go) | Assemble a streamed function call, execute addition, and return its result | Uses a fixed `8 + 13` conversation |
| [structured_output](structured_output/main.go) | Configure strict JSON schema and decode the answer | Optional `ARK_PROMPT` |
| [reasoning](reasoning/main.go) | Stream reasoning and pass the assistant message back for a follow-up | Uses a fixed multiplication/follow-up conversation |
| [modifiers](modifiers/main.go) | Modify request JSON, inspect original response bytes, and finalize a stream | Optional `ARK_PROMPT` |
| [multimodal](multimodal/main.go) | Build image/audio/video message parts, or read a local image as base64 | `ARK_IMAGE_URL`, `ARK_AUDIO_URL`, `ARK_VIDEO_URL`; optional `ARK_IMAGE_FILE` and `ARK_PROMPT` |
| [file](file/main.go) | Send a PDF by FileID or local base64 | `ARK_FILE_ID` or `ARK_FILE_PATH`; optional `ARK_PROMPT` |
| [upload](upload/main.go) | Upload a PDF, wait for processing, send its FileID, and delete the temporary file | Optional `ARK_FILE_PATH` and `ARK_PROMPT` |
| [tool_result](tool_result/main.go) | Return a document FileID in a native multimodal tool result | Required `ARK_FILE_ID`; optional `ARK_PROMPT` |
| [session_cache](session_cache/main.go) | Store a response and reuse its cached context | Uses a fixed recall conversation |
| [prefix_cache](prefix_cache/main.go) | Create a reusable prefix and continue from its response ID | Uses a fixed prefix and recall conversation |
| [web_search](web_search/main.go) | Configure the built-in search tool | Optional `ARK_PROMPT` |

The multimodal example uses remote media when their URL variables are present.
Otherwise it reads `ARK_IMAGE_FILE`, defaulting to the repository's EINO logo.
The file and upload examples default to the synthetic
[marker PDF](testdata/marker.pdf), containing `PDF_OK_731`. An existing
`ARK_FILE_ID` is never deleted; the upload example deletes only its own upload,
including on processing or inference errors.

Thinking is disabled in the shared defaults. The reasoning example enables it
explicitly in its own model configuration. Sampling, tools, schemas, prompts,
and other feature settings are shown in the individual examples so they can be
copied or edited directly.

Cache, search, and media understanding require model/account support. In
particular, an accepted tool-result request does not establish that the model
read its document correctly. The examples demonstrate API usage; automated
output checks belong to the separate validation harness and adapter tests.
