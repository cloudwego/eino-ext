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
| [stream](stream/main.go) | Receive deltas, concatenate, and check/print the assembled message structure | Optional `ARK_PROMPT` |
| [tools](tools/main.go) | Force a streamed call without naming the sole tool, execute addition, and return its result | Uses a fixed `8 + 13` conversation |
| [tool_options](tool_options/main.go) | Combine strict defaults/overrides, full JSON Schema, bound/call-level tools, and Allowed/Forced name subsets | Uses fixed addition and multiplication prompts |
| [structured_output](structured_output/main.go) | Configure strict JSON schema and decode the answer | Optional `ARK_PROMPT` |
| [reasoning](reasoning/main.go) | Stream reasoning and pass the assistant message back for a follow-up | Uses a fixed multiplication/follow-up conversation |
| [modifiers](modifiers/main.go) | Modify request JSON, inspect original response bytes, and finalize a stream | Optional `ARK_PROMPT` |
| [multimodal](multimodal/main.go) | Build image/audio/video message parts, or read a local image as base64 | `ARK_IMAGE_URL`, `ARK_AUDIO_URL`, `ARK_VIDEO_URL`; optional `ARK_IMAGE_FILE` and `ARK_PROMPT` |
| [file](file/main.go) | Send a PDF by FileID or local base64 | `ARK_FILE_ID` or `ARK_FILE_PATH`; optional `ARK_PROMPT` |
| [upload](upload/main.go) | Upload a PDF, wait for processing, send its FileID, and delete the temporary file | Optional `ARK_FILE_PATH` and `ARK_PROMPT` |
| [tool_result](tool_result/main.go) | Return a document FileID in a native multimodal tool result | Required `ARK_FILE_ID`; optional `ARK_PROMPT` |
| [cached_tools](cached_tools/main.go) | Reuse the same Forced multi-name options across cached calls, then request a final answer | Uses a fixed `(8 + 13) * 2` conversation |
| [cached_tool_options](cached_tool_options/main.go) | Reuse the same Allowed multi-name options across a cached tool result and ordinary follow-up | Uses a fixed `8 + 13` conversation |
| [session_cache](session_cache/main.go) | Store a response and reuse its cached context | Uses a fixed recall conversation |
| [prefix_cache](prefix_cache/main.go) | Create a reusable prefix and continue from its response ID | Uses a fixed prefix and recall conversation |
| [web_search](web_search/main.go) | Require the web-content plugin and inspect native search items and usage | Optional `ARK_PROMPT` |
| [web_search_doubao](web_search_doubao/main.go) | Select Doubao Search Custom with `sources: ["doubao"]` and inspect its execution | Optional `ARK_PROMPT` |

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
read its document correctly. The stream, reasoning and tools examples check the concatenated parts, metadata,
usage and function-call structure, then print the assembled message. Encrypted
signatures are abbreviated in the printed copy; the original message is used
for subsequent calls. Model-answer checks belong to the separate validation
harness and adapter tests.

The [tool options example](tool_options/main.go) shows a full parameter schema
with `additionalProperties: false`. `add` inherits the model's strict default,
`multiply` overrides it with false, and `subtract` is excluded by candidate names.
Allowed permits a direct answer; Forced requires a call. A single explicit name
selects that function; Forced without a name or with multiple names uses
`required`. The basic tools example leaves strict unset.

The [forced cached tools example](cached_tools/main.go) keeps the same model,
complete Eino history, and Forced candidate options across two function calls.
It invalidates local cache references before applying Forbidden in a new chain
to finish with a plain answer. The
[allowed cached tools example](cached_tool_options/main.go) keeps its options
unchanged across all three calls, including the ordinary follow-up. Tools are
sent on the first call and inherited on continuations; `tools` and `tool_choice`
are both omitted from cached requests. Response IDs and cached input token counts
are printed. Change tool definitions, the candidate set, or the tool-choice policy
by starting a new cache chain. Both examples and the session/prefix examples
require the cache service to be enabled for the selected account and model.

The `web_search` example requires the **web-content plugin**. The
`web_search_doubao` example sets `sources: ["doubao"]` and requires **Doubao Search
Custom**. These are separate activations in the console. Both require a search
call and print native search item status and service usage, so a URL in the
answer alone is not mistaken for an executed search.

The [message parts example](message_parts/main.go) edits returned text parts,
changes the aggregate compatibility fields to unrelated values, persists the
message through JSON, and continues with Stream. The model receives the edited
parts. The first data part carries item metadata; other parts reference its
item-index. Transfer item-level Extra before deleting that data part. Reasoning
summaries retain their native summary indices. One encrypted signature belongs
to the item and is carried by its first summary part. Item and tool positions
remain independent of stream merge indices. Added/delta/done chunks belonging
to the same SDK part reuse one stream merge index. `ReasoningContent` displays
only summaries; raw reasoning text remains in the structured output parts.
Function call item metadata is stored in `ToolCall.Extra`; function calls do not
create empty output parts.
